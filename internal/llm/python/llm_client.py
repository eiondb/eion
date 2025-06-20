"""
LLM Client - OpenAI integration for knowledge extraction
Provides language model capabilities for entity and relationship extraction
"""

import asyncio
import json
import logging
import traceback
import os
from abc import ABC, abstractmethod
from typing import Dict, List, Optional, Any, Type
from dataclasses import asdict
import inspect
import re

import httpx
from tenacity import retry, stop_after_attempt, wait_random_exponential, retry_if_exception
from pydantic import BaseModel

import sys
import os
sys.path.append(os.path.join(os.path.dirname(__file__), '..', '..', '..'))
from internal.knowledge.python.knowledge_models import Message

logger = logging.getLogger(__name__)

DEFAULT_TEMPERATURE = 0
DEFAULT_MAX_TOKENS = 2000
MULTILINGUAL_EXTRACTION_RESPONSES = (
    '\n\nAny extracted information should be returned in the same language as it was written in.'
)


class RateLimitError(Exception):
    """Rate limit error - exactly from Eion Knowledge"""
    pass


def is_server_or_retry_error(exception):
    """Error detection function - exactly from Eion Knowledge"""
    if isinstance(exception, (RateLimitError, json.decoder.JSONDecodeError)):
        return True

    return (
        isinstance(exception, httpx.HTTPStatusError) and 500 <= exception.response.status_code < 600
    )


class LLMClient(ABC):
    """Abstract LLM Client - migrated exactly from Eion Knowledge"""
    
    def __init__(self, api_key: Optional[str] = None, model: str = "gpt-4", temperature: float = DEFAULT_TEMPERATURE, max_tokens: int = DEFAULT_MAX_TOKENS):
        self.api_key = api_key or os.getenv("OPENAI_API_KEY")
        self.model = model
        self.temperature = temperature
        self.max_tokens = max_tokens
        
        if not self.api_key:
            raise ValueError("OpenAI API key not provided")

    def _clean_input(self, input_str: str) -> str:
        """Clean input string - exactly from Eion Knowledge"""
        # Clean any invalid Unicode
        cleaned = input_str.encode('utf-8', errors='ignore').decode('utf-8')

        # Remove zero-width characters and other invisible unicode
        zero_width = '\u200b\u200c\u200d\ufeff\u2060'
        for char in zero_width:
            cleaned = cleaned.replace(char, '')

        # Remove control characters except newlines, returns, and tabs
        cleaned = ''.join(char for char in cleaned if ord(char) >= 32 or char in '\n\r\t')

        return cleaned

    @retry(
        stop=stop_after_attempt(4),
        wait=wait_random_exponential(multiplier=10, min=5, max=120),
        retry=retry_if_exception(is_server_or_retry_error),
        after=lambda retry_state: logger.warning(
            f'Retrying {retry_state.fn.__name__ if retry_state.fn else "function"} after {retry_state.attempt_number} attempts...'
        ) if retry_state.attempt_number > 1 else None,
        reraise=True,
    )
    async def _generate_response_with_retry(
        self,
        messages: List[Message],
        response_model: Optional[Type[BaseModel]] = None,
        max_tokens: int = DEFAULT_MAX_TOKENS,
    ) -> Dict[str, Any]:
        """Generate response with retry - exactly from Eion Knowledge"""
        try:
            return await self._generate_response(messages, response_model, max_tokens)
        except (httpx.HTTPStatusError, RateLimitError) as e:
            raise e

    @abstractmethod
    async def _generate_response(
        self,
        messages: List[Message],
        response_model: Optional[Type[BaseModel]] = None,
        max_tokens: int = DEFAULT_MAX_TOKENS,
    ) -> Dict[str, Any]:
        """Abstract method for generating response"""
        pass

    async def generate_response(
        self,
        messages: List[Message],
        response_model: Optional[Type[BaseModel]] = None,
        max_tokens: Optional[int] = None,
    ) -> Dict[str, Any]:
        """Generate response - exactly from Eion Knowledge"""
        if max_tokens is None:
            max_tokens = self.max_tokens

        # Add Pydantic schema to prompt if response_model provided
        if response_model is not None:
            serialized_model = json.dumps(response_model.model_json_schema())
            messages[-1].content += (
                f'\n\nRespond with a JSON object in the following format:\n\n{serialized_model}'
            )

        # Add multilingual extraction instructions
        messages[0].content += MULTILINGUAL_EXTRACTION_RESPONSES

        # Clean input messages
        for message in messages:
            message.content = self._clean_input(message.content)

        response = await self._generate_response_with_retry(
            messages, response_model, max_tokens
        )

        return response


class OpenAIClient(LLMClient):
    """OpenAI LLM Client - implements Eion Knowledge's OpenAI functionality"""
    
    def __init__(self, api_key: Optional[str] = None, model: str = "gpt-4", **kwargs):
        super().__init__(api_key, model, **kwargs)
        self.base_url = "https://api.openai.com/v1"
    
    async def _generate_response(
        self,
        messages: List[Message],
        response_model: Optional[Type[BaseModel]] = None,
        max_tokens: int = DEFAULT_MAX_TOKENS,
    ) -> Dict[str, Any]:
        """Generate response using OpenAI API"""
        
        # Convert messages to OpenAI format
        openai_messages = []
        for msg in messages:
            openai_messages.append({
                "role": msg.role,
                "content": msg.content
            })
        
        # Prepare request payload
        payload = {
            "model": self.model,
            "messages": openai_messages,
            "temperature": self.temperature,
            "max_tokens": max_tokens,
        }
        
        # If response_model is provided, use structured output
        if response_model:
            payload["response_format"] = {"type": "json_object"}
        
        # Make API request
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json"
        }
        
        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(
                    f"{self.base_url}/chat/completions",
                    json=payload,
                    headers=headers,
                    timeout=60.0
                )
                response.raise_for_status()
                
                result = response.json()
                content = result["choices"][0]["message"]["content"]
                
                # Parse JSON response if structured output was requested
                if response_model:
                    try:
                        return json.loads(content)
                    except json.JSONDecodeError as e:
                        logger.error(f"Failed to parse JSON response: {content}")
                        raise e
                
                return {"content": content}
                
            except httpx.HTTPStatusError as e:
                if e.response.status_code == 429:
                    raise RateLimitError("Rate limit exceeded")
                raise e
            except httpx.RequestError as e:
                raise RuntimeError(f"Request failed: {e}")


# Factory function to create LLM client - matches Eion Knowledge pattern
def create_llm_client(provider: str = "openai", **kwargs) -> LLMClient:
    """Create LLM client - factory pattern matching Eion Knowledge"""
    if provider.lower() == "openai":
        return OpenAIClient(**kwargs)
    else:
        raise ValueError(f"Unsupported LLM provider: {provider}")


# Default client instance
LLMClient = create_llm_client 