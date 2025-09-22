from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class QuestionRequest(_message.Message):
    __slots__ = ("question", "user_id", "material_ids", "context")
    class ContextEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    MATERIAL_IDS_FIELD_NUMBER: _ClassVar[int]
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    question: str
    user_id: str
    material_ids: _containers.RepeatedScalarFieldContainer[str]
    context: _containers.ScalarMap[str, str]
    def __init__(self, question: _Optional[str] = ..., user_id: _Optional[str] = ..., material_ids: _Optional[_Iterable[str]] = ..., context: _Optional[_Mapping[str, str]] = ...) -> None: ...

class SourceReference(_message.Message):
    __slots__ = ("material_id", "content_snippet", "relevance_score")
    MATERIAL_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_SNIPPET_FIELD_NUMBER: _ClassVar[int]
    RELEVANCE_SCORE_FIELD_NUMBER: _ClassVar[int]
    material_id: str
    content_snippet: str
    relevance_score: float
    def __init__(self, material_id: _Optional[str] = ..., content_snippet: _Optional[str] = ..., relevance_score: _Optional[float] = ...) -> None: ...

class QuestionResponse(_message.Message):
    __slots__ = ("answer", "confidence", "sources", "metadata")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    SOURCES_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    answer: str
    confidence: float
    sources: _containers.RepeatedCompositeFieldContainer[SourceReference]
    metadata: _containers.ScalarMap[str, str]
    def __init__(self, answer: _Optional[str] = ..., confidence: _Optional[float] = ..., sources: _Optional[_Iterable[_Union[SourceReference, _Mapping]]] = ..., metadata: _Optional[_Mapping[str, str]] = ...) -> None: ...

class SearchRequest(_message.Message):
    __slots__ = ("query", "user_id", "top_k")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    TOP_K_FIELD_NUMBER: _ClassVar[int]
    query: str
    user_id: str
    top_k: int
    def __init__(self, query: _Optional[str] = ..., user_id: _Optional[str] = ..., top_k: _Optional[int] = ...) -> None: ...

class SearchResult(_message.Message):
    __slots__ = ("material_id", "content", "similarity_score", "metadata")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MATERIAL_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    SIMILARITY_SCORE_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    material_id: str
    content: str
    similarity_score: float
    metadata: _containers.ScalarMap[str, str]
    def __init__(self, material_id: _Optional[str] = ..., content: _Optional[str] = ..., similarity_score: _Optional[float] = ..., metadata: _Optional[_Mapping[str, str]] = ...) -> None: ...

class SearchResponse(_message.Message):
    __slots__ = ("results",)
    RESULTS_FIELD_NUMBER: _ClassVar[int]
    results: _containers.RepeatedCompositeFieldContainer[SearchResult]
    def __init__(self, results: _Optional[_Iterable[_Union[SearchResult, _Mapping]]] = ...) -> None: ...

class EmbeddingRequest(_message.Message):
    __slots__ = ("content", "material_id", "content_type")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    MATERIAL_ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    content: str
    material_id: str
    content_type: str
    def __init__(self, content: _Optional[str] = ..., material_id: _Optional[str] = ..., content_type: _Optional[str] = ...) -> None: ...

class EmbeddingResponse(_message.Message):
    __slots__ = ("embedding", "embedding_id")
    EMBEDDING_FIELD_NUMBER: _ClassVar[int]
    EMBEDDING_ID_FIELD_NUMBER: _ClassVar[int]
    embedding: _containers.RepeatedScalarFieldContainer[float]
    embedding_id: str
    def __init__(self, embedding: _Optional[_Iterable[float]] = ..., embedding_id: _Optional[str] = ...) -> None: ...
