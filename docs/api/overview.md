# API Reference

SPYDER exposes a REST API on the metrics port (default `:9090`) for runtime control, monitoring, and querying discovered data.

## Interactive Reference

<OASpec :hideInfo="false" :groupByTags="true" :hideBranding="true" />

## Authentication

All endpoints require an API key passed via the `Authorization` header:

```http
Authorization: Bearer <api-key>
```

Keys have one of three scopes: `read`, `write`, or `admin`. See [Authentication](/api/authentication) for details.

## Base URL

```
http://localhost:9090/api/v1/
```
