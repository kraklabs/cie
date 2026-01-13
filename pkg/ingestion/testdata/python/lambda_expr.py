"""Sample lambda expressions."""

# Lambda as variable
double = lambda x: x * 2

# Lambda in function
def apply_operation(x, operation=lambda y: y * 2):
    """Apply operation to x."""
    return operation(x)

# Lambda in map
numbers = [1, 2, 3, 4, 5]
squared = list(map(lambda x: x ** 2, numbers))
