package docs

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OpenAPI specification served at /openapi.json
const openapiJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "arkstudy Gateway API",
    "version": "0.1.0"
  },
  "servers": [ { "url": "/" } ],
  "tags": [
    {"name": "auth", "description": "Authentication"},
    {"name": "users", "description": "User Management"},
    {"name": "materials", "description": "Materials"},
    {"name": "processing", "description": "Processing Results"},
    {"name": "ai", "description": "LLM and Search"}
  ],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      }
    }
  },
  "paths": {
    "/api/register": {
      "post": {
        "summary": "Register a user",
        "tags": ["auth"],
        "requestBody": {"required": true},
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/api/login": {
      "post": {"summary": "Login","tags": ["auth"],"responses": {"200": {"description": "OK"}}}
    },
    "/api/validate": {
      "get": {"summary": "Validate token","tags": ["auth"],"responses": {"200": {"description": "OK"}}}
    },
    "/api/users": {
      "get": {"summary": "List users","tags": ["users"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/users/{id}": {
      "get": {"summary": "Get user by ID","tags": ["users"],"security": [{"bearerAuth": []}],"parameters": [{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/users/username/{username}": {
      "get": {"summary": "Get user by username","tags": ["users"],"security": [{"bearerAuth": []}],"parameters": [{"name":"username","in":"path","required":true,"schema":{"type":"string"}}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/users/email/{email}": {
      "get": {"summary": "Get user by email","tags": ["users"],"security": [{"bearerAuth": []}],"parameters": [{"name":"email","in":"path","required":true,"schema":{"type":"string"}}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/materials/upload": {
      "post": {"summary": "Upload material","tags": ["materials"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/materials": {
      "get": {"summary": "List materials","tags": ["materials"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/materials/{id}": {
      "get": {"summary": "Get material by ID","tags": ["materials"],"security": [{"bearerAuth": []}],"parameters": [{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses": {"200": {"description": "OK"}}},
      "delete": {"summary": "Delete material","tags": ["materials"],"security": [{"bearerAuth": []}],"parameters": [{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/materials/process": {
      "post": {"summary": "Process material","tags": ["materials"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/processing/results": {
      "get": {"summary": "List processing results","tags": ["processing"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/processing/results/{material_id}": {
      "get": {"summary": "Get processing result by material ID","tags": ["processing"],"security": [{"bearerAuth": []}],"parameters": [{"name":"material_id","in":"path","required":true,"schema":{"type":"string"}}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/processing/results/{task_id}": {
      "put": {"summary": "Update processing result","tags": ["processing"],"security": [{"bearerAuth": []}],"parameters": [{"name":"task_id","in":"path","required":true,"schema":{"type":"string"}}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/ai/ask": {
      "post": {"summary": "Ask LLM","tags": ["ai"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/ai/ask/stream": {
      "get": {"summary": "Ask LLM stream","tags": ["ai"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}},
      "post": {"summary": "Ask LLM stream","tags": ["ai"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    },
    "/api/ai/search": {
      "get": {"summary": "Semantic search","tags": ["ai"],"security": [{"bearerAuth": []}],"responses": {"200": {"description": "OK"}}}
    }
  }
}`

// RegisterRoutes wires the API documentation endpoints into the Gin engine.
// - GET /openapi.json: OpenAPI 3.0 spec
// - GET /docs: Swagger UI (via CDN) loading /openapi.json
func RegisterRoutes(r *gin.Engine) {
	// convenience: redirect root to docs
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/docs") })
	r.GET("/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(openapiJSON))
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
	})
}

// Simple Swagger-UI page using CDN assets, pointing to /openapi.json
const swaggerHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>arkstudy API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>body { margin: 0; padding: 0; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '/openapi.json',
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis],
      layout: 'BaseLayout'
    });
  </script>
 </body>
</html>`
