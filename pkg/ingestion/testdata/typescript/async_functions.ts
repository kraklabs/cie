// Async functions
export async function fetchData(url: string): Promise<any> {
    const response = await fetch(url);
    return response.json();
}

export async function processItems(items: string[]): Promise<string[]> {
    return Promise.all(items.map(async (item) => {
        return await fetchData(item);
    }));
}
