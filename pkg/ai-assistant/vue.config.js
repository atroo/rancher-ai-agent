const path = require('path');

// Root of the extension repo (two levels up from pkg/ai-assistant/)
const rootDir = path.resolve(__dirname, '..', '..');
const shellDir = path.resolve(rootDir, 'node_modules', '@rancher', 'shell');

module.exports = {
  chainWebpack(config) {
    // Resolve @shell/* imports to @rancher/shell
    config.resolve.alias.set('@shell', shellDir);

    // Resolve @rancher/auto-import
    config.resolve.alias.set('@rancher/auto-import', path.join(shellDir, 'pkg', 'auto-import'));

    // Resolve @pkg/* imports
    config.resolve.alias.set('@pkg', path.resolve(rootDir, 'pkg'));

    // Allow resolving .ts files
    config.resolve.extensions.add('.ts');

    // Add ts-loader for .ts files
    config.module
      .rule('ts')
      .test(/\.ts$/)
      .use('ts-loader')
      .loader('ts-loader')
      .options({
        transpileOnly:     true,
        appendTsSuffixTo:  [/\.vue$/],
        configFile:        path.resolve(rootDir, 'tsconfig.json'),
      });
  },
};
