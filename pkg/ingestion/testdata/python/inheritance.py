"""Sample class inheritance."""

class Animal:
    """Base animal class."""

    def __init__(self, name):
        self.name = name

    def speak(self):
        """Make a sound."""
        pass

class Dog(Animal):
    """Dog class inheriting from Animal."""

    def speak(self):
        """Dog barks."""
        return "Woof!"

    def fetch(self):
        """Dog fetches."""
        return "Fetching!"

class Cat(Animal):
    """Cat class inheriting from Animal."""

    def speak(self):
        """Cat meows."""
        return "Meow!"
