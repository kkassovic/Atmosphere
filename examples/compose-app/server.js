const express = require('express');
const redis = require('redis');

const app = express();
const port = process.env.PORT || 3000;

// Redis client
const redisClient = redis.createClient({
  url: 'redis://redis:6379'
});

redisClient.connect().catch(console.error);

// Routes
app.get('/', async (req, res) => {
  try {
    // Increment view counter
    const views = await redisClient.incr('views');
    
    res.send(`
      <!DOCTYPE html>
      <html lang="en">
      <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>Atmosphere Compose App</title>
        <style>
          body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
          }
          .container {
            text-align: center;
            padding: 2rem;
            background: rgba(255, 255, 255, 0.1);
            border-radius: 20px;
            backdrop-filter: blur(10px);
          }
          h1 { font-size: 3rem; margin-bottom: 0.5rem; }
          p { font-size: 1.2rem; opacity: 0.9; }
          .badge {
            display: inline-block;
            padding: 0.5rem 1rem;
            background: rgba(255, 255, 255, 0.2);
            border-radius: 50px;
            margin-top: 1rem;
            font-size: 1.5rem;
          }
        </style>
      </head>
      <body>
        <div class="container">
          <h1>🚀 Atmosphere</h1>
          <p>Docker Compose app with Redis</p>
          <div class="badge">Views: ${views}</div>
          <p style="margin-top: 2rem; opacity: 0.7;">
            Node.js + Express + Redis
          </p>
        </div>
      </body>
      </html>
    `);
  } catch (error) {
    res.status(500).send('Error connecting to Redis: ' + error.message);
  }
});

app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});

app.listen(port, '0.0.0.0', () => {
  console.log(`Server running on port ${port}`);
});
