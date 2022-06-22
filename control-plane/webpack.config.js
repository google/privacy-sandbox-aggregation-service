const mode = process.env.NODE_ENV === 'production' ? 'production' : 'development'
const MiniCssExtractPlugin = require('mini-css-extract-plugin');

module.exports = {
    mode: mode,
    module: {
        rules: [
            {
                test: /\.(png|jpe?g|gif|svg)$/i,
                type: "asset/resource"
            },
            {
                test: /\.css$/i,
                use: [
                    {
                        loader: MiniCssExtractPlugin.loader,
                        options: {
                            publicPath: "",
                        }
                    },
                    "css-loader"
                ],
            },
            {
                test: /\.js$/,
                exclude: /node_modules/,
                use: {
                    loader: "babel-loader"
                }
            }
        ]
    },
    plugins: [new MiniCssExtractPlugin()],
    devtool: 'source-map',
    devServer: {
        static: './dist'
    }
}