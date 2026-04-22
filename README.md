# Best Buy Makeline Service

This service is a background worker responsible for processing customer orders in the Best Buy Cloud-Native Application.

## Responsibilities
- Monitor new orders in MongoDB
- Update order status (e.g., Pending → Processing → Completed)
- Simulate order fulfillment workflow

## Tech Stack
- Go
- MongoDB

## Related Services
- order-service (creates orders)
- product-service (product data)
- store-front (customer UI)
- store-admin (admin UI)

## Deployment
This service is containerized using Docker and deployed to AKS using Kubernetes.