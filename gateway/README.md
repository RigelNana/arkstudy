# Gateway API Docs

- Swagger UI: visit `/docs` on the gateway base URL (e.g. http://localhost:8080/docs)
- OpenAPI JSON: `/openapi.json`

## Authorize
- Click the "Authorize" button in Swagger UI
- Enter `Bearer <your-jwt-token>`

## Notes
- Most `/api/*` routes require JWT. Obtain it from `/api/login` after `/api/register`.
- For streaming `/api/ai/ask/stream`, use GET or POST and keep the connection open.

## gRPC Services (reflection enabled)
You can browse and call gRPC endpoints using grpcui.

Examples:
- Auth Service: `grpcui -plaintext localhost:50051`
- User Service: `grpcui -plaintext localhost:50052`
- Material Service: `grpcui -plaintext localhost:50053`
- OCR Service: `grpcui -plaintext localhost:50055`

Replace host/port with your actual service addresses.
