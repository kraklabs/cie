// Arrow functions
export const double = (x: number) => x * 2;

export const greet = (name: string): string => {
    return `Hello, ${name}!`;
};

const processArray = (arr: number[]) => {
    return arr.map(x => x * 2);
};
