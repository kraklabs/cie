// Module exports
export function publicFunction() {}
function privateFunction() {}

export default class DefaultExport {
    method() {}
}

export { privateFunction as renamedExport };
