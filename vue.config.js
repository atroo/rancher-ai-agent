const path = require('path');

module.exports = {
  chainWebpack(config) {
    const shellDir = path.resolve(__dirname, 'node_modules', '@rancher', 'shell');

    // Resolve @shell/* imports to @rancher/shell
    config.resolve.alias.set('@shell', shellDir);

    // Resolve @rancher/auto-import
    config.resolve.alias.set('@rancher/auto-import', path.join(shellDir, 'pkg', 'auto-import'));

    // Resolve @pkg/* imports
    config.resolve.alias.set('@pkg', path.resolve(__dirname, 'pkg'));
  },
};
