FROM node

COPY . /app

WORKDIR /app
# 更换淘宝源
RUN npm config set registry http://registry.npm.taobao.org
RUN npm install -g pnpm
RUN pnpm config set registry http://registry.npm.taobao.org
RUN pnpm install
RUN pnpm run build

CMD ["pnpm", "preview", "--host", "0.0.0.0"]

