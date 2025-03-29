module.exports = {
    root: true,
    env: {
        node: true,
        es6: true  // Add ES6 support
    },
    parserOptions: {
        parser: '@babel/eslint-parser',
        ecmaVersion: 2020,
        sourceType: 'module'
    },
    rules: {
        'no-console': 'off',
        'no-debugger': 'off'
    },
    // Disable eslint for all files
    ignorePatterns: ['**/*']
}