FROM node:20-alpine

WORKDIR /app

COPY package.json .
COPY package-lock.json .
COPY . .

RUN npm install

RUN npm run build

EXPOSE 5173

CMD ["npm", "run", "dev"]