"""Sample functions with type hints."""
from typing import List, Dict, Optional

def process_list(items: List[int]) -> List[int]:
    """Process a list of integers."""
    return [x * 2 for x in items]

def get_config() -> Dict[str, str]:
    """Get configuration dictionary."""
    return {"key": "value"}

def find_user(user_id: int) -> Optional[str]:
    """Find user by ID, returns None if not found."""
    if user_id > 0:
        return "user"
    return None
