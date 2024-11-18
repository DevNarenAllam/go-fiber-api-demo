package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/swagger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "players.com/m/docs"
)

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Redis and GORM global clients
var (
	redisClient *redis.Client
	db          *gorm.DB
	ctx         = context.Background()
)

func initRedis() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // No password
		DB:       0,  // Default DB
	})

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}

	return client
}

type Player struct {
	PlayerID     string     `gorm:"column:playerID;type:varchar(10);primaryKey" json:"playerID"`
	BirthYear    *int       `gorm:"column:birthYear;type:int" json:"birthYear"`
	BirthMonth   *int       `gorm:"column:birthMonth;type:int" json:"birthMonth"`
	BirthDay     *int       `gorm:"column:birthDay;type:int" json:"birthDay"`
	BirthCountry *string    `gorm:"column:birthCountry;type:varchar(50)" json:"birthCountry"`
	BirthState   *string    `gorm:"column:birthState;type:varchar(50)" json:"birthState"`
	BirthCity    *string    `gorm:"column:birthCity;type:varchar(50)" json:"birthCity"`
	DeathYear    *int       `gorm:"column:deathYear;type:int" json:"deathYear"`
	DeathMonth   *int       `gorm:"column:deathMonth;type:int" json:"deathMonth"`
	DeathDay     *int       `gorm:"column:deathDay;type:int" json:"deathDay"`
	DeathCountry *string    `gorm:"column:deathCountry;type:varchar(50)" json:"deathCountry"`
	DeathState   *string    `gorm:"column:deathState;type:varchar(50)" json:"deathState"`
	DeathCity    *string    `gorm:"column:deathCity;type:varchar(50)" json:"deathCity"`
	NameFirst    *string    `gorm:"column:nameFirst;type:varchar(50)" json:"nameFirst"`
	NameLast     *string    `gorm:"column:nameLast;type:varchar(50)" json:"nameLast"`
	NameGiven    *string    `gorm:"column:nameGiven;type:varchar(50)" json:"nameGiven"`
	Weight       *int       `gorm:"column:weight;type:int" json:"weight"`
	Height       *int       `gorm:"column:height;type:int" json:"height"`
	Bats         *string    `gorm:"column:bats;type:char(1)" json:"bats"`
	Throws       *string    `gorm:"column:throws;type:char(1)" json:"throws"`
	Debut        *time.Time `gorm:"column:debut;type:date" json:"debut"`
	FinalGame    *time.Time `gorm:"column:finalGame;type:date" json:"finalGame"`
	RetroID      *string    `gorm:"column:retroID;type:varchar(10)" json:"retroID"`
	BbrefID      *string    `gorm:"column:bbrefID;type:varchar(10)" json:"bbrefID"`
}

// TableName explicitly sets the name of the table to match the existing schema
func (Player) TableName() string {
	return "players"
}

// Rate Limiting and Throttling Configurations
const (
	RATE_LIMIT       = 50
	RATE_PERIOD      = 60
	THROTTLE_LIMIT   = 50
	THROTTLE_PERIOD  = 60
	COOLDOWN_PERIOD  = 30
	RedisRateLimit   = "rate_limit:"
	RedisThrottleKey = "throttle:"
)

//	@title			Players API
//	@version		1.0
//	@description	This is a sample server for managing players.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	http://www.swagger.io/support
//	@contact.email	support@swagger.io

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@host		localhost:3000
//	@BasePath	/

//	@securityDefinitions.basic	BasicAuth

func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Initialize Redis and Database
	redisClient = initRedis()
	defer redisClient.Close()

	var err error
	// MySQL DSN: username:password@tcp(host:port)/dbname
	dsn := "naren:Python#123@tcp(localhost:3306)/players?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect database")
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to obtain database instance")
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	db.AutoMigrate(&Player{})

	// Fiber instance and CORS middleware
	app := fiber.New()
	app.Use(cors.New())

	// Middleware for rate limiting
	app.Use(rateLimitMiddleware)
	app.Use(throttleMiddleware)

	// define an end point to return all players with caching
	app.Get("/players", cache("players", time.Minute*5, getPlayers))
	// define an end point to return a range of players
	app.Get("/players/range", getPlayersRange)
	// define an end point to return a single player
	app.Get("/players/:playerID", getPlayerByID)

	app.Get("/swagger/*", swagger.HandlerDefault)
	// Health check endpoint
	app.Get("/health", healthCheck)

	// Start server with graceful shutdown
	go func() {
		if err := app.Listen(":3000"); err != nil {
			log.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	// Graceful shutdown
	gracefulShutdown(app)
}

// Rate limiting middleware
func rateLimitMiddleware(c *fiber.Ctx) error {
	clientIP := c.IP()
	key := RedisRateLimit + clientIP

	val, err := redisClient.Get(ctx, key).Int()
	if err == redis.Nil {
		redisClient.Set(ctx, key, 1, time.Duration(RATE_PERIOD)*time.Second)
	} else if err == nil {
		if val >= RATE_LIMIT {
			return fiber.NewError(fiber.StatusTooManyRequests, "Rate limit exceeded")
		}
		redisClient.Incr(ctx, key)
	} else {
		return err
	}
	return c.Next()
}

// Throttle middleware
func throttleMiddleware(c *fiber.Ctx) error {
	clientIP := c.IP()
	key := RedisThrottleKey + clientIP

	data, err := redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		resetTime := time.Now().Add(time.Second * THROTTLE_PERIOD).Unix()
		redisClient.Set(ctx, key, fmt.Sprintf("1:%d", resetTime), time.Duration(THROTTLE_PERIOD)*time.Second)
	} else if err == nil {
		count, resetTimeStr, _ := parseThrottleData(data)
		resetTime, _ := strconv.ParseInt(resetTimeStr, 10, 64)

		if time.Now().Unix() < resetTime && count >= THROTTLE_LIMIT {
			return fiber.NewError(fiber.StatusTooManyRequests, "Throttle limit exceeded")
		}

		if time.Now().Unix() >= resetTime {
			count = 0
			resetTime = time.Now().Add(time.Second * COOLDOWN_PERIOD).Unix()
		}

		count++
		redisClient.Set(ctx, key, fmt.Sprintf("%d:%d", count, resetTime), time.Duration(THROTTLE_PERIOD)*time.Second)
	} else {
		return err
	}
	return c.Next()
}

// Helper function to parse throttle data
func parseThrottleData(data string) (count int, resetTimeStr string, err error) {
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		err = fmt.Errorf("invalid throttle data format")
		return
	}
	count, err = strconv.Atoi(parts[0])
	if err != nil {
		return
	}
	resetTimeStr = parts[1]
	return
}

// Cache middleware - Example caching function
func cache(key string, expiration time.Duration, next fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		val, err := redisClient.Get(ctx, key).Bytes()
		if err == nil && len(val) > 0 {
			c.Response().SetBody(val)
			return nil
		}
		if err == redis.Nil {
			if err := next(c); err != nil {
				return err
			}
			redisClient.Set(ctx, key, c.Response().Body(), expiration)
		}
		return nil
	}
}

//	@Summary		Get all players
//	@Description	Get a list of all players
//	@Tags			players
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}	Player
//	@Router			/players [get]
//
// Handler to get all players with cache
func getPlayers(c *fiber.Ctx) error {
	var players []Player
	db.Find(&players)
	//fmt.Println(players)
	return c.JSON(players)
}

// @Summary Get a range of players
// @Description Get players with pagination using offset and limit
// @Tags players
// @Accept json
// @Produce json
// @Param offset query int false "Offset for pagination" default(0)
// @Param limit query int false "Number of players to return" default(10)
// @Success 200 {array} Player
// @Failure 400 {object} ErrorResponse
// @Router /players/range [get]
func getPlayersRange(c *fiber.Ctx) error {
	// Get offset and limit from query parameters with defaults
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid offset parameter")
	}

	limit, err := strconv.Atoi(c.Query("limit", "10"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid limit parameter")
	}

	// Validate limit
	if limit <= 0 || limit > 100 {
		return fiber.NewError(fiber.StatusBadRequest, "Limit must be between 1 and 100")
	}

	var players []Player
	result := db.Offset(offset).Limit(limit).Find(&players)
	if result.Error != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}

	return c.JSON(players)
}

//	@Summary		Get player by ID
//	@Description	Get a player by their ID
//	@Tags			players
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Player ID"
//	@Success		200	{object}	Player
//	@Failure		400	{object}	error	"Invalid player ID"
//	@Failure		404	{object}	error	"Player not found"
//	@Router			/players/{id} [get]
//
// Handler to get player by ID with cache
func getPlayerByID(c *fiber.Ctx) error {
	id := c.Params("playerID")
	var player Player
	if err := db.First(&player, "playerID = ?", id).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Player not found")
	}
	return c.JSON(player)
}

//	@Summary		Health check
//	@Description	Check the health of the service
//	@Tags			health
//	@Accept			json
//	@Produce		json
//	@Success		200	{string}	string		"OK"
//	@Failure		503	{object}	error	"Service unavailable"
//	@Router			/health [get]
//
// Health check handler
func healthCheck(c *fiber.Ctx) error {
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "Redis not available")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "Database not available")
	}
	if err := sqlDB.Ping(); err != nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "Database not available")
	}
	return c.SendString("OK")
}

// Graceful shutdown
func gracefulShutdown(app *fiber.App) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	if err := app.Shutdown(); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}
