from __future__ import annotations

import logging
import re
from typing import Dict, Any, List
from dataclasses import dataclass

import io
from minio import Minio
from pypdf import PdfReader
import docx
from app.config import get_settings
from app.core.chunker import chunk_text
from app.core.embedding import embed_text
from app.core.pg_vector_store import pg_vector_store


logger = logging.getLogger(__name__)


@dataclass
class DocumentChunk:
    """文档块结构"""
    content: str
    type: str = "text"  # text, heading, table, code, image
    language: str = "zh"
    level: int = 0  # 标题层级
    metadata: Dict[str, Any] | None = None


class DocumentProcessor:
    """智能文档处理器"""
    
    def __init__(self):
        self.settings = get_settings()
        self.minio_client = Minio(
            self.settings.minio_endpoint,
            access_key=self.settings.minio_access_key,
            secret_key=self.settings.minio_secret_key,
            secure=False
        )
        
    async def extract_text_from_minio(self, file_path: str, file_type: str) -> str:
        """从 MinIO 提取文本内容"""
        try:
            logger.info(f"Extracting text from {file_path} (type: {file_type})")
            
            response = self.minio_client.get_object(self.settings.minio_bucket_name, file_path)
            file_data = response.read()
            response.close()
            response.release_conn()

            if file_type.lower() in ['pdf']:
                return self._extract_pdf_content(file_data)
            elif file_type.lower() in ['docx', 'doc']:
                return self._extract_docx_content(file_data)
            elif file_type.lower() in ['txt', 'md']:
                return self._extract_text_content(file_data)
            else:
                logger.warning(f"Unsupported file type: {file_type}")
                return ""
                
        except Exception as e:
            logger.error(f"Error extracting text from {file_path}: {e}")
            return ""
    
    def _extract_pdf_content(self, file_data: bytes) -> str:
        """提取 PDF 内容"""
        text = ""
        with io.BytesIO(file_data) as f:
            reader = PdfReader(f)
            for page in reader.pages:
                text += page.extract_text()
        return text
    
    def _extract_docx_content(self, file_data: bytes) -> str:
        """提取 DOCX 内容"""
        text = ""
        with io.BytesIO(file_data) as f:
            doc = docx.Document(f)
            for para in doc.paragraphs:
                text += para.text + "\n"
        return text
    
    def _extract_text_content(self, file_data: bytes) -> str:
        """提取纯文本内容"""
        return file_data.decode('utf-8')
    
    async def intelligent_chunking(
        self, 
        content: str, 
        file_type: str, 
        options: Dict[str, Any] | None = None
    ) -> List[DocumentChunk]:
        """智能分块策略"""
        options = options or {}
        max_chunk_size = options.get('max_chunk_size', 1000)
        overlap_size = options.get('overlap_size', 200)
        
        chunks = []
        
        try:
            # 1. 识别文档结构
            structured_content = await self._analyze_document_structure(content, file_type)
            
            # 2. 根据结构进行分块
            for section in structured_content:
                section_chunks = await self._chunk_section(
                    section, 
                    max_chunk_size, 
                    overlap_size
                )
                chunks.extend(section_chunks)
                
        except Exception as e:
            logger.error(f"Error in intelligent chunking: {e}")
            # 降级到简单分块
            simple_chunks = chunk_text(content, max_tokens=max_chunk_size)
            chunks = [
                DocumentChunk(
                    content=chunk,
                    type="text",
                    metadata={'chunk_method': 'simple'}
                ) 
                for chunk in simple_chunks
            ]
        
        return chunks
    
    async def _analyze_document_structure(self, content: str, file_type: str) -> List[Dict[str, Any]]:
        """分析文档结构"""
        sections = []
        
        # 简单的标题识别
        lines = content.split('\n')
        current_section = {'content': '', 'type': 'text', 'level': 0}
        
        for line in lines:
            line = line.strip()
            if not line:
                continue
                
            # 识别标题（简单规则）
            heading_level = self._detect_heading_level(line)
            
            if heading_level > 0:
                # 保存当前段落
                if current_section['content']:
                    sections.append(current_section)
                
                # 开始新段落
                current_section = {
                    'content': line,
                    'type': 'heading',
                    'level': heading_level
                }
            else:
                current_section['content'] += '\n' + line
        
        # 添加最后一个段落
        if current_section['content']:
            sections.append(current_section)
        
        return sections
    
    def _detect_heading_level(self, line: str) -> int:
        """检测标题层级"""
        # Markdown 风格标题
        if line.startswith('#'):
            return len(line) - len(line.lstrip('#'))
        
        # 数字标题 (1. 2.1 等)
        if re.match(r'^\d+\.', line):
            return 1
        if re.match(r'^\d+\.\d+', line):
            return 2
        if re.match(r'^\d+\.\d+\.\d+', line):
            return 3
            
        # 中文标题（一、二、三）
        if re.match(r'^[一二三四五六七八九十]+、', line):
            return 1
        if re.match(r'^[（(]?[一二三四五六七八九十]+[）)]', line):
            return 2
            
        return 0
    
    async def _chunk_section(
        self, 
        section: Dict[str, Any], 
        max_size: int, 
        overlap: int
    ) -> List[DocumentChunk]:
        """对单个段落进行分块"""
        content = section['content']
        section_type = section.get('type', 'text')
        level = section.get('level', 0)
        
        # 如果内容较短，直接返回
        if len(content) <= max_size:
            return [DocumentChunk(
                content=content,
                type=section_type,
                level=level,
                metadata={'section_type': section_type}
            )]
        
        # 对长内容进行分块
        chunks = []
        sentences = re.split(r'[。！？；\n]', content)
        current_chunk = ""
        
        for sentence in sentences:
            if not sentence.strip():
                continue
                
            # 检查添加当前句子是否超出限制
            if len(current_chunk) + len(sentence) > max_size and current_chunk:
                # 保存当前块
                chunks.append(DocumentChunk(
                    content=current_chunk.strip(),
                    type=section_type,
                    level=level,
                    metadata={'section_type': section_type, 'chunk_method': 'sentence'}
                ))
                
                # 开始新块，保留重叠
                if overlap > 0 and len(current_chunk) > overlap:
                    current_chunk = current_chunk[-overlap:] + sentence
                else:
                    current_chunk = sentence
            else:
                current_chunk += sentence + "。"
        
        # 添加最后一块
        if current_chunk.strip():
            chunks.append(DocumentChunk(
                content=current_chunk.strip(),
                type=section_type,
                level=level,
                metadata={'section_type': section_type, 'chunk_method': 'sentence'}
            ))
        
        return chunks
    
    async def extract_structured_content(
        self, 
        content: str, 
        file_type: str, 
        options: Dict[str, Any] | None = None
    ) -> Dict[str, Any]:
        """结构化内容提取"""
        try:
            # 提取标题
            headings = self._extract_headings(content)
            
            # 提取关键信息
            keywords = await self._extract_keywords(content)
            
            # 提取摘要
            summary = await self._generate_summary(content, options)
            
            return {
                'headings': headings,
                'keywords': keywords,
                'summary': summary,
                'content_length': len(content),
                'estimated_reading_time': len(content) // 300,  # 假设每分钟300字
            }
            
        except Exception as e:
            logger.error(f"Error in structured content extraction: {e}")
            return {}
    
    def _extract_headings(self, content: str) -> List[Dict[str, Any]]:
        """提取标题"""
        headings = []
        lines = content.split('\n')
        
        for i, line in enumerate(lines):
            line = line.strip()
            level = self._detect_heading_level(line)
            if level > 0:
                headings.append({
                    'text': line,
                    'level': level,
                    'line_number': i + 1
                })
        
        return headings
    
    async def _extract_keywords(self, content: str) -> List[str]:
        """提取关键词"""
        # 简单的关键词提取（实际可以使用 NLP 库）
        import re
        from collections import Counter
        
        # 移除标点符号，分词
        words = re.findall(r'[\u4e00-\u9fff]+', content)  # 中文词汇
        
        # 过滤停用词和短词
        stopwords = {'的', '是', '在', '有', '和', '与', '及', '等', '这', '那', '个', '了', '中', '为', '上', '下'}
        words = [w for w in words if len(w) >= 2 and w not in stopwords]
        
        # 计算词频，返回前20个
        word_counts = Counter(words)
        return [word for word, count in word_counts.most_common(20)]
    
    async def _generate_summary(self, content: str, options: Dict[str, Any] | None = None) -> str:
        """生成摘要"""
        options = options or {}
        max_summary_length = options.get('summary_length', 200)
        
        # 简单的摘要生成（取前几句）
        sentences = re.split(r'[。！？]', content)
        summary = ""
        
        for sentence in sentences:
            sentence = sentence.strip()
            if not sentence:
                continue
                
            if len(summary) + len(sentence) > max_summary_length:
                break
            summary += sentence + "。"
        
        return summary.strip()
    
    async def analyze_document(
        self, 
        content: str, 
        options: Dict[str, Any] | None = None
    ) -> Dict[str, Any]:
        """AI 文档分析"""
        try:
            # 基础统计
            char_count = len(content)
            word_count = len(re.findall(r'[\u4e00-\u9fff]', content))
            
            # 内容分析
            keywords = await self._extract_keywords(content)
            summary = await self._generate_summary(content, options)
            headings = self._extract_headings(content)
            
            # 简单的内容分类
            content_type = self._classify_content(content)
            
            # 难度评估
            difficulty = self._assess_difficulty(content)
            
            return {
                'statistics': {
                    'char_count': char_count,
                    'word_count': word_count,
                    'paragraph_count': len(content.split('\n\n')),
                    'heading_count': len(headings)
                },
                'content_analysis': {
                    'type': content_type,
                    'difficulty': difficulty,
                    'keywords': keywords[:10],  # 前10个关键词
                    'summary': summary
                },
                'structure': {
                    'headings': headings,
                    'has_structure': len(headings) > 0
                }
            }
            
        except Exception as e:
            logger.error(f"Error in document analysis: {e}")
            return {}
    
    def _classify_content(self, content: str) -> str:
        #TODO: 结合更复杂的模型进行分类
        """简单的内容分类"""
        # 基于关键词的简单分类
        if any(word in content for word in ['数学', '计算', '公式', '证明']):
            return 'mathematics'
        elif any(word in content for word in ['编程', '代码', '算法', '函数']):
            return 'programming'
        elif any(word in content for word in ['历史', '朝代', '年代', '事件']):
            return 'history'
        elif any(word in content for word in ['物理', '化学', '生物', '实验']):
            return 'science'
        else:
            return 'general'
    
    def _assess_difficulty(self, content: str) -> str:
        #TODO: 结合更复杂的模型进行评估
        """评估内容难度"""
        # 简单的难度评估
        word_count = len(re.findall(r'[\u4e00-\u9fff]', content))
        
        # 计算平均句长
        sentences = re.split(r'[。！？]', content)
        avg_sentence_length = word_count / max(len(sentences), 1)
        
        # 检查专业词汇
        technical_words = len(re.findall(r'[专业术语模式]', content))  # 需要更精确的模式
        
        if avg_sentence_length > 30 or technical_words > 10:
            return 'advanced'
        elif avg_sentence_length > 20 or technical_words > 5:
            return 'intermediate'
        else:
            return 'beginner'
    
    async def process_document(
        self,
        file_path: str,
        file_id: str,
        user_id: str,
        file_type: str
    ) -> List[str]:
        """完整的文档处理流程"""
        try:
            logger.info(f"Processing document {file_id} from {file_path}")
            
            # 1. 提取文本内容
            content = await self.extract_text_from_minio(file_path, file_type)
            if not content.strip():
                logger.warning(f"No content extracted from {file_path}")
                return []
            
            # 2. 智能分块
            chunks = await self.intelligent_chunking(content, file_type)
            if not chunks:
                logger.warning(f"No chunks created from {file_path}")
                return []
            
            # 3. 向量化和存储
            chunk_ids = []
            for i, chunk in enumerate(chunks):
                try:
                    # 生成嵌入向量
                    embedding = embed_text(chunk.content)
                    
                    # 准备元数据
                    metadata = {
                        'chunk_index': i,
                        'total_chunks': len(chunks),
                        'file_type': file_type,
                        'difficulty': chunk.metadata.get('difficulty', 'beginner') if chunk.metadata else 'beginner',
                        'subject': chunk.metadata.get('subject', '') if chunk.metadata else '',
                        'tags': chunk.metadata.get('tags', []) if chunk.metadata else [],
                        'level': chunk.level,
                        'language': chunk.language
                    }
                    
                    # 存储到向量数据库
                    chunk_id = await pg_vector_store.add_chunk(
                        content=chunk.content,
                        embedding=embedding,
                        metadata=metadata,
                        chunk_type=chunk.type,
                        file_id=file_id,
                        user_id=user_id
                    )
                    
                    chunk_ids.append(chunk_id)
                    logger.debug(f"Stored chunk {i+1}/{len(chunks)} with ID {chunk_id}")
                    
                except Exception as e:
                    logger.error(f"Error processing chunk {i}: {e}")
                    continue
            
            logger.info(f"Successfully processed {len(chunk_ids)} chunks for file {file_id}")
            return chunk_ids
            
        except Exception as e:
            logger.error(f"Error processing document {file_id}: {e}")
            raise
         
    async def process_text(
        self,
        content: str,
        file_id: str,
        user_id: str,
        file_type: str
    ) -> List[str]:
        """完整的纯文本处理流程"""
        try:
            logger.info(f"Processing text for file {file_id}")

            if not content.strip():
                logger.warning(f"No content provided for {file_id}")
                return []

            # 2. 智能分块
            chunks = await self.intelligent_chunking(content, file_type)
            if not chunks:
                logger.warning(f"No chunks created from {file_id}")
                return []

            # 3. 向量化和存储
            chunk_ids = []
            for i, chunk in enumerate(chunks):
                try:
                    # 生成嵌入向量
                    embedding = embed_text(chunk.content)
                    
                    # 准备元数据
                    metadata = {
                        'chunk_index': i,
                        'total_chunks': len(chunks),
                        'file_type': file_type,
                        'difficulty': chunk.metadata.get('difficulty', 'beginner') if chunk.metadata else 'beginner',
                        'subject': chunk.metadata.get('subject', '') if chunk.metadata else '',
                        'tags': chunk.metadata.get('tags', []) if chunk.metadata else [],
                        'level': chunk.level,
                        'language': chunk.language
                    }
                    
                    # 存储到向量数据库
                    chunk_id = await pg_vector_store.add_chunk(
                        content=chunk.content,
                        embedding=embedding,
                        metadata=metadata,
                        chunk_type=chunk.type,
                        file_id=file_id,
                        user_id=user_id
                    )
                    
                    chunk_ids.append(chunk_id)
                    logger.debug(f"Stored chunk {i+1}/{len(chunks)} with ID {chunk_id}")
                    
                except Exception as e:
                    logger.error(f"Error processing chunk {i}: {e}")
                    continue
            
            logger.info(f"Successfully processed {len(chunk_ids)} chunks for file {file_id}")
            return chunk_ids
            
        except Exception as e:
            logger.error(f"Error processing text for file {file_id}: {e}")
            raise