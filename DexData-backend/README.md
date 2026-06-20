# Go Backend

This service accepts workshop form submissions and stores them in MongoDB.

## Endpoints

- `GET /healthz`
- `POST /api/leads`

## Request body

```json
{
  "fullName": "Priya Sharma",
  "phone": "9876543210",
  "email": "priya@schoolname.edu",
  "school": "DPS School",
  "city": "Mumbai",
  "source": "dexguru-workshop"
}
```

## Setup

1. Copy `.env.example` values into your shell environment.
2. Set `MONGODB_URI` to your MongoDB Atlas cluster URI.
3. Run:

```bash
go mod tidy
go run .
```

The API runs on `http://localhost:8080` by default.
