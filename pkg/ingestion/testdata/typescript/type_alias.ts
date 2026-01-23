// Type aliases
export type UserId = number;
export type Handler<T> = (value: T) => void;
export type Result<T, E> = { ok: true; value: T } | { ok: false; error: E };
