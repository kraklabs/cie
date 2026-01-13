// Interface definitions
export interface User {
    id: number;
    name: string;
    email: string;
}

export interface Repository<T> {
    find(id: number): T | null;
    save(item: T): void;
}
