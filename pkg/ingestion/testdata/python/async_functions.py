"""Sample async functions."""

async def fetch_data(url: str):
    """Fetch data from URL."""
    # Simulated async operation
    return await simulate_request(url)

async def simulate_request(url: str):
    """Simulate HTTP request."""
    return {"url": url, "data": "test"}

async def process_items(items):
    """Process items asynchronously."""
    results = []
    for item in items:
        result = await fetch_data(item)
        results.append(result)
    return results
