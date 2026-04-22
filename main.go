package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Valid database API types
const (
	AZURE_COSMOS_DB_SQL_API = "cosmosdbsql"
)

func main() {
	var orderService *OrderService
	var err error

	// Get the database API type
	apiType := os.Getenv("ORDER_DB_API")
	switch apiType {
	case AZURE_COSMOS_DB_SQL_API:
		log.Printf("Using Azure CosmosDB SQL API")
	default:
		log.Printf("Using MongoDB API")
	}

	// Initialize the database
	orderService, err = initDatabase(apiType)
	if err != nil {
		log.Printf("Failed to initialize database: %s", err)
		os.Exit(1)
	}

	router := gin.Default()
	router.Use(cors.Default())
	router.Use(OrderMiddleware(orderService))

	router.GET("/order/fetch", fetchOrders)
	router.GET("/order/:id", getOrder)
	router.PUT("/order", updateOrder)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": os.Getenv("APP_VERSION"),
		})
	})

	log.Println("Best Buy Makeline Service started on port 3001")
	router.Run(":3001")
}

// OrderMiddleware injects the order service into the request context
func OrderMiddleware(orderService *OrderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("orderService", orderService)
		c.Next()
	}
}

// Fetches and processes pending orders from MongoDB
func fetchOrders(c *gin.Context) {
	client, ok := c.MustGet("orderService").(*OrderService)
	if !ok {
		log.Printf("Failed to get order service")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Get pending orders from DB
	orders, err := client.repo.GetPendingOrders()
	if err != nil {
		log.Printf("Failed to get pending orders from database: %s", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Process each pending order
	for _, order := range orders {
		order.Status = Processing
		err := client.repo.UpdateOrder(order)
		if err != nil {
			log.Printf("Failed to update order %s to Processing: %s", order.OrderID, err)
			continue
		}

		order.Status = Complete
		err = client.repo.UpdateOrder(order)
		if err != nil {
			log.Printf("Failed to update order %s to Complete: %s", order.OrderID, err)
			continue
		}
	}

	c.IndentedJSON(http.StatusOK, orders)
}

// Gets a single order from database by order ID
func getOrder(c *gin.Context) {
	client, ok := c.MustGet("orderService").(*OrderService)
	if !ok {
		log.Printf("Failed to get order service")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("Failed to convert order id to int: %s", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	sanitizedOrderId := strconv.FormatInt(int64(id), 10)

	order, err := client.repo.GetOrder(sanitizedOrderId)
	if err != nil {
		log.Printf("Failed to get order from database: %s", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.IndentedJSON(http.StatusOK, order)
}

// Updates the status of an order
func updateOrder(c *gin.Context) {
	client, ok := c.MustGet("orderService").(*OrderService)
	if !ok {
		log.Printf("Failed to get order service")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var order Order
	if err := c.BindJSON(&order); err != nil {
		log.Printf("Failed to unmarshal order: %s", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	id, err := strconv.Atoi(order.OrderID)
	if err != nil {
		log.Printf("Failed to convert order id to int: %s", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	sanitizedOrderId := strconv.FormatInt(int64(id), 10)

	sanitizedOrder := Order{
		OrderID:    sanitizedOrderId,
		CustomerID: order.CustomerID,
		Items:      order.Items,
		Status:     order.Status,
	}

	err = client.repo.UpdateOrder(sanitizedOrder)
	if err != nil {
		log.Printf("Failed to update order status: %s", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Order updated successfully",
	})
}

// Gets an environment variable or exits if it is not set
func getEnvVar(varName string, fallbackVarNames ...string) string {
	value := os.Getenv(varName)
	if value == "" {
		for _, fallbackVarName := range fallbackVarNames {
			value = os.Getenv(fallbackVarName)
			if value != "" {
				break
			}
		}
		if value == "" {
			log.Printf("%s is not set", varName)
			if len(fallbackVarNames) > 0 {
				log.Printf("Tried fallback variables: %v", fallbackVarNames)
			}
			os.Exit(1)
		}
	}
	return value
}

// Initializes the database based on the API type
func initDatabase(apiType string) (*OrderService, error) {
	dbURI := getEnvVar("AZURE_COSMOS_RESOURCEENDPOINT", "ORDER_DB_URI")
	dbName := getEnvVar("ORDER_DB_NAME")

	switch apiType {
	case AZURE_COSMOS_DB_SQL_API:
		containerName := getEnvVar("ORDER_DB_CONTAINER_NAME")
		dbPartitionKey := getEnvVar("ORDER_DB_PARTITION_KEY")
		dbPartitionValue := getEnvVar("ORDER_DB_PARTITION_VALUE")

		useWorkloadIdentityAuth := os.Getenv("USE_WORKLOAD_IDENTITY_AUTH")
		if useWorkloadIdentityAuth == "" {
			useWorkloadIdentityAuth = "false"
		}

		if useWorkloadIdentityAuth == "true" {
			cosmosRepo, err := NewCosmosDBOrderRepoWithManagedIdentity(
				dbURI,
				dbName,
				containerName,
				PartitionKey{dbPartitionKey, dbPartitionValue},
			)
			if err != nil {
				return nil, err
			}
			return NewOrderService(cosmosRepo), nil
		}

		dbPassword := os.Getenv("ORDER_DB_PASSWORD")
		cosmosRepo, err := NewCosmosDBOrderRepo(
			dbURI,
			dbName,
			containerName,
			dbPassword,
			PartitionKey{dbPartitionKey, dbPartitionValue},
		)
		if err != nil {
			return nil, err
		}
		return NewOrderService(cosmosRepo), nil

	default:
		collectionName := getEnvVar("ORDER_DB_COLLECTION_NAME")
		dbUsername := os.Getenv("ORDER_DB_USERNAME")
		dbPassword := os.Getenv("ORDER_DB_PASSWORD")

		mongoRepo, err := NewMongoDBOrderRepo(
			dbURI,
			dbName,
			collectionName,
			dbUsername,
			dbPassword,
		)
		if err != nil {
			return nil, err
		}
		return NewOrderService(mongoRepo), nil
	}
}