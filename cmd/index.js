require('dotenv').config();

const polygonApiKey = process.env.POLYGON_API_KEY;
const databaseUrl = process.env.DATABASE_URL;

console.log(`Polygon API Key: ${polygonApiKey}`);
console.log(`Database URL: ${databaseUrl}`);
