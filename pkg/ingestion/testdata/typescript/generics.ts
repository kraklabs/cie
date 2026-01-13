// Generic functions and classes
export function identity<T>(value: T): T {
    return value;
}

export class Container<T> {
    constructor(private value: T) {}

    getValue(): T {
        return this.value;
    }
}
