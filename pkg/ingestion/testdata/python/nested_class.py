"""Sample nested class."""

class Outer:
    """Outer class."""

    def __init__(self):
        self.value = 10

    class Inner:
        """Inner nested class."""

        def __init__(self, parent_value):
            self.parent_value = parent_value

        def get_value(self):
            """Get parent value."""
            return self.parent_value

    def create_inner(self):
        """Create inner instance."""
        return self.Inner(self.value)
