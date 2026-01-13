"""Sample class with methods."""

class UserService:
    """Service for managing users."""

    def __init__(self, db):
        self.db = db

    def get_user(self, user_id: int):
        """Get user by ID."""
        return self.db.query(user_id)

    def create_user(self, name: str):
        """Create a new user."""
        return self.db.insert(name)
