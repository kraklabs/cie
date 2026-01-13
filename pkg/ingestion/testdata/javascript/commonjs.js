// CommonJS module
const helper = require('./helper');

function process(data) {
    return helper.transform(data);
}

module.exports = { process };
