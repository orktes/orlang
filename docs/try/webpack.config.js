const webpack = require('webpack');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const path = require('path');

module.exports = {
  entry: './index.js',
  output: {
    path: path.join(__dirname, './build'),
    filename: 'index.js',
  },
  module: {
    loaders: [
      {
        test: /\.go$/,
        loader: 'gopherjs-loader'
      },
      {
        test: /\.html$/,
        loader: 'file?name=[name].[ext]',
      },
      {
        test: /\.json$/,
        loader: 'json',
      },
      {
        test: /\.(js|jsx)$/,
        exclude: /node_modules/,
        loaders: [
          "react-hot-loader",
          "babel-loader"
        ],
      },
    ],
  },
  plugins: [
    new webpack.HotModuleReplacementPlugin(),
    new webpack.optimize.OccurrenceOrderPlugin(),
    new webpack.DefinePlugin({
      'process.env': { NODE_ENV: JSON.stringify(process.env.NODE_ENV || 'development') },
    }),
    new webpack.SourceMapDevToolPlugin({
      exclude: /node_modules/,
    }),
    new CopyWebpackPlugin([
      {
        from: 'node_modules/monaco-editor/min/vs',
        to: 'vs',
      }
    ]),
    new CopyWebpackPlugin([
      {
        from: 'index.html',
        to: 'index.html',
      }
    ]),
  ],
  devServer: {
    contentBase: './',
    hot: true,
  },
}
