"""Sample decorated functions and classes."""

def cache(func):
    """Cache decorator."""
    def wrapper(*args, **kwargs):
        return func(*args, **kwargs)
    return wrapper

@cache
def expensive_operation(x):
    """Expensive operation that should be cached."""
    return x * 2

@cache
def another_operation(x, y):
    """Another cached operation."""
    return x + y
