from __future__ import annotations

import asyncio
import json
import logging
from typing import Dict, Any, Optional
import traceback

from aiokafka import AIOKafkaConsumer
from aiokafka.errors import KafkaError

from app.config import get_settings
from app.services.document_processor import DocumentProcessor
from app.core.pg_vector_store import pg_vector_store

logger = logging.getLogger(__name__)


class KafkaFileProcessor:
    """Kafka 文件处理器"""
    
    def __init__(self):
        self.settings = get_settings()
        self.processor = DocumentProcessor()
        self.running = False
        self.consumer: Optional[AIOKafkaConsumer] = None
        self.consumer_task: Optional[asyncio.Task] = None
    
    async def start(self):
        """启动处理器"""
        try:
            self.running = True
            
            # 获取 Kafka 配置
            kafka_brokers = self.settings.kafka_bootstrap_servers
            kafka_topic = self.settings.kafka_topic_file_processing
            kafka_group_id = self.settings.kafka_group_id
            
            logger.info(f"Starting Kafka consumer for topic '{kafka_topic}' at {kafka_brokers}")
            
            # 创建 Kafka 消费者
            self.consumer = AIOKafkaConsumer(
                kafka_topic,
                bootstrap_servers=kafka_brokers,
                group_id=kafka_group_id,
                value_deserializer=lambda x: json.loads(x.decode('utf-8')),
                enable_auto_commit=True,
                auto_commit_interval_ms=1000,
                auto_offset_reset='latest'  # 只处理新消息
            )
            
            # 启动消费者
            await self.consumer.start()
            
            # 启动消费循环
            self.consumer_task = asyncio.create_task(self._consume_messages())
            
            logger.info("Kafka file processor started successfully")
            
        except Exception as e:
            logger.error(f"Error starting Kafka file processor: {e}")
            self.running = False
            raise
    
    async def stop(self):
        """停止处理器"""
        try:
            self.running = False
            
            # 停止消费任务
            if self.consumer_task:
                self.consumer_task.cancel()
                try:
                    await self.consumer_task
                except asyncio.CancelledError:
                    pass
            
            # 停止消费者
            if self.consumer:
                await self.consumer.stop()
            
            logger.info("Kafka file processor stopped")
            
        except Exception as e:
            logger.error(f"Error stopping Kafka file processor: {e}")
    
    async def _consume_messages(self):
        """消费 Kafka 消息的主循环"""
        try:
            async for message in self.consumer:
                if not self.running:
                    break
                
                try:
                    # 处理消息
                    await self._handle_message(message.value)
                    
                except Exception as e:
                    logger.error(f"Error processing message: {e}")
                    # 继续处理下一条消息
                    
        except Exception as e:
            logger.error(f"Error in message consumption loop: {e}")
    
    async def _handle_message(self, message_data: Dict[str, Any]):
        """处理单个消息"""
        try:
            logger.info(f"Processing file message: {message_data}")
            
            # 提取消息信息
            file_id = message_data.get('file_id')
            file_path = message_data.get('file_path')
            
            if not file_id or not file_path:
                logger.warning("Invalid message: missing file_id or file_path")
                return
            
            # 检查是否已处理过
            existing_chunks = await pg_vector_store.get_chunks_by_file(file_id)
            if existing_chunks:
                logger.info(f"File {file_id} already processed, skipping")
                return
            
            # 处理文档
            chunks = await self.process_file_message(message_data)
            
            logger.info(f"Successfully processed file {file_id}: {len(chunks) if chunks else 0} chunks created")
            
        except Exception as e:
            logger.error(f"Error handling message {message_data}: {e}")
            raise
    
    async def process_file_message(self, message_data: Dict[str, Any]):
        """处理文件消息"""
        try:
            logger.info(f"Processing file message: {message_data}")
            
            # 提取消息信息
            file_id = message_data.get('file_id')
            file_path = message_data.get('file_path')
            user_id = message_data.get('user_id')
            file_type = message_data.get('file_type', 'unknown')
            
            if not file_id or not file_path:
                logger.warning("Invalid message: missing file_id or file_path")
                return
            
            # 检查是否已处理过
            existing_chunks = await pg_vector_store.get_chunks_by_file(file_id)
            if existing_chunks:
                logger.info(f"File {file_id} already processed, skipping")
                return
            
            # 处理文档
            chunks = await self.processor.process_document(
                file_path=file_path,
                file_id=file_id,
                user_id=user_id,
                file_type=file_type
            )
            
            logger.info(f"Processed file {file_id}: {len(chunks)} chunks created")
            return chunks
            
        except Exception as e:
            logger.error(f"Error processing file message {message_data}: {e}")
            raise


# 全局实例
kafka_file_processor = KafkaFileProcessor()