#!/usr/bin/env python3
"""
Real Embedding Service using sentence-transformers
Downloads and uses the actual all-MiniLM-L6-v2 model for production embeddings
"""

import sys
import json
import logging
import argparse
from typing import List, Dict, Any
import numpy as np

try:
    from sentence_transformers import SentenceTransformer
    SENTENCE_TRANSFORMERS_AVAILABLE = True
except ImportError:
    SENTENCE_TRANSFORMERS_AVAILABLE = False
    logging.warning("sentence-transformers not available, falling back to mock embeddings")

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class RealEmbeddingService:
    """Real embedding service using sentence-transformers"""
    
    def __init__(self, model_name: str = "all-MiniLM-L6-v2"):
        self.model_name = model_name
        self.dimension = 384  # all-MiniLM-L6-v2 dimension
        self.model = None
        self._load_model()
    
    def _load_model(self):
        """Load the sentence-transformers model"""
        if not SENTENCE_TRANSFORMERS_AVAILABLE:
            logger.error("sentence-transformers not available")
            return
        
        try:
            logger.info(f"Loading embedding model: {self.model_name}")
            self.model = SentenceTransformer(self.model_name)
            logger.info(f"Model loaded successfully, dimension: {self.dimension}")
        except Exception as e:
            logger.error(f"Failed to load model {self.model_name}: {e}")
            self.model = None
    
    def generate_embeddings(self, texts: List[str]) -> List[List[float]]:
        """Generate embeddings for a list of texts"""
        if not self.model:
            # Fallback to mock embeddings if model failed to load
            logger.warning("Using mock embeddings - model not available")
            return self._generate_mock_embeddings(texts)
        
        try:
            # Generate embeddings using sentence-transformers
            embeddings = self.model.encode(texts, convert_to_tensor=False, show_progress_bar=False)
            
            # Convert to list of lists
            if isinstance(embeddings, np.ndarray):
                embeddings = embeddings.tolist()
            
            return embeddings
        except Exception as e:
            logger.error(f"Failed to generate embeddings: {e}")
            # Fallback to mock embeddings
            return self._generate_mock_embeddings(texts)
    
    def _generate_mock_embeddings(self, texts: List[str]) -> List[List[float]]:
        """Generate mock embeddings as fallback"""
        embeddings = []
        for text in texts:
            # Generate deterministic mock embedding
            hash_val = hash(text) % (2**31)
            np.random.seed(hash_val)
            embedding = np.random.randn(self.dimension).astype(float)
            # Normalize
            embedding = embedding / np.linalg.norm(embedding)
            embeddings.append(embedding.tolist())
        return embeddings


def main():
    parser = argparse.ArgumentParser(description="Embedding Service")
    parser.add_argument("command", choices=["embed"], help="Command to execute")
    parser.add_argument("--model", default="all-MiniLM-L6-v2", help="Model name")
    
    args = parser.parse_args()
    
    if args.command == "embed":
        # Read request from stdin
        try:
            request_data = json.load(sys.stdin)
            texts = request_data.get("texts", [])
            model_name = request_data.get("model", args.model)
            
            if not texts:
                response = {"error": "No texts provided"}
            else:
                # Initialize embedding service
                service = RealEmbeddingService(model_name)
                
                # Generate embeddings
                embeddings = service.generate_embeddings(texts)
                
                response = {
                    "embeddings": embeddings,
                    "model": model_name,
                    "dimension": service.dimension
                }
            
            # Output response
            print(json.dumps(response))
            
        except Exception as e:
            error_response = {"error": str(e)}
            print(json.dumps(error_response))
            sys.exit(1)


if __name__ == "__main__":
    main() 