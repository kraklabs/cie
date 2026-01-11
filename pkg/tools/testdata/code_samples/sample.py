"""
Sample Python code for testing parsing and indexing.
"""

from typing import Optional, List
from dataclasses import dataclass


@dataclass
class User:
    """Represents a user in the system."""
    id: str
    name: str
    email: str


class UserRepository:
    """Handles user data operations."""

    def __init__(self, database):
        self.database = database

    def get_user(self, user_id: str) -> Optional[User]:
        """Retrieve a user by ID.

        Args:
            user_id: The user identifier

        Returns:
            User object if found, None otherwise
        """
        if not user_id:
            raise ValueError("user_id cannot be empty")

        result = self.database.query(
            "SELECT * FROM users WHERE id = ?",
            (user_id,)
        )

        if not result:
            return None

        return User(**result[0])

    def create_user(self, user: User) -> bool:
        """Create a new user.

        Args:
            user: User object to create

        Returns:
            True if successful, False otherwise
        """
        if not user:
            raise ValueError("user cannot be None")

        self.database.insert("users", {
            "id": user.id,
            "name": user.name,
            "email": user.email
        })

        return True

    def list_users(self) -> List[User]:
        """List all users.

        Returns:
            List of User objects
        """
        results = self.database.query("SELECT * FROM users")
        return [User(**row) for row in results]


def process_users(users: List[User]) -> dict:
    """Process a list of users and return statistics.

    Args:
        users: List of users to process

    Returns:
        Dictionary with user statistics
    """
    return {
        "total": len(users),
        "emails": [u.email for u in users],
        "names": [u.name for u in users]
    }
