# HTTP contracts

These OpenAPI 3.0 documents are the source for the generated frontend types.
They intentionally cover the endpoints used by the current admin and storefront
applications; expand them alongside new BFF routes.

Generate the TypeScript types:

```bash
cd frontend
pnpm generate:api
```

The generated files live in `packages/api/src/generated/`. Do not edit them by
hand. CI regenerates and checks that the committed output is current.
