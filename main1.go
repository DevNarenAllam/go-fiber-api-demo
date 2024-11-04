package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/go-redis/redis/v8"
// 	"github.com/gofiber/fiber/v2"
// 	"github.com/gofiber/fiber/v2/middleware/cors"
// 	"gorm.io/driver/mysql"
// 	"gorm.io/gorm"
// )

// // Redis and GORM global clients
// var (
// 	redisClient *redis.Client
// 	db          *gorm.DB
// 	ctx         = context.Background()
// )

// // Initialize Redis client
// func initRedis() *redis.Client {
// 	client := redis.NewClient(&redis.Options{
// 		Addr:     "localhost:6379",
// 		Password: "", // No password
// 		DB:       0,  // Default DB
// 	})
// 	return client
// }

// type Player struct {
// 	PlayerID     string     `gorm:"column:playerID;type:varchar(10);primaryKey" json:"playerID"`
// 	BirthYear    *int       `gorm:"column:birthYear;type:int" json:"birthYear,omitempty"`
// 	BirthMonth   *int       `gorm:"column:birthMonth;type:int" json:"birthMonth,omitempty"`
// 	BirthDay     *int       `gorm:"column:birthDay;type:int" json:"birthDay,omitempty"`
// 	BirthCountry *string    `gorm:"column:birthCountry;type:varchar(50)" json:"birthCountry,omitempty"`
// 	BirthState   *string    `gorm:"column:birthState;type:varchar(50)" json:"birthState,omitempty"`
// 	BirthCity    *string    `gorm:"column:birthCity;type:varchar(50)" json:"birthCity,omitempty"`
// 	DeathYear    *int       `gorm:"column:deathYear;type:int" json:"deathYear,omitempty"`
// 	DeathMonth   *int       `gorm:"column:deathMonth;type:int" json:"deathMonth,omitempty"`
// 	DeathDay     *int       `gorm:"column:deathDay;type:int" json:"deathDay,omitempty"`
// 	DeathCountry *string    `gorm:"column:deathCountry;type:varchar(50)" json:"deathCountry,omitempty"`
// 	DeathState   *string    `gorm:"column:deathState;type:varchar(50)" json:"deathState,omitempty"`
// 	DeathCity    *string    `gorm:"column:deathCity;type:varchar(50)" json:"deathCity,omitempty"`
// 	NameFirst    *string    `gorm:"column:nameFirst;type:varchar(50)" json:"nameFirst,omitempty"`
// 	NameLast     *string    `gorm:"column:nameLast;type:varchar(50)" json:"nameLast,omitempty"`
// 	NameGiven    *string    `gorm:"column:nameGiven;type:varchar(50)" json:"nameGiven,omitempty"`
// 	Weight       *int       `gorm:"column:weight;type:int" json:"weight,omitempty"`
// 	Height       *int       `gorm:"column:height;type:int" json:"height,omitempty"`
// 	Bats         *string    `gorm:"column:bats;type:char(1)" json:"bats,omitempty"`
// 	Throws       *string    `gorm:"column:throws;type:char(1)" json:"throws,omitempty"`
// 	Debut        *time.Time `gorm:"column:debut;type:date" json:"debut,omitempty"`
// 	FinalGame    *time.Time `gorm:"column:finalGame;type:date" json:"finalGame,omitempty"`
// 	RetroID      *string    `gorm:"column:retroID;type:varchar(10)" json:"retroID,omitempty"`
// 	BbrefID      *string    `gorm:"column:bbrefID;type:varchar(10)" json:"bbrefID,omitempty"`
// }

// // TableName explicitly sets the name of the table to match the existing schema
// func (Player) TableName() string {
// 	return "players"
// }

// // Rate Limiting and Throttling Configurations
// const (
// 	RATE_LIMIT       = 5
// 	RATE_PERIOD      = 60
// 	THROTTLE_LIMIT   = 5
// 	THROTTLE_PERIOD  = 60
// 	COOLDOWN_PERIOD  = 30
// 	RedisRateLimit   = "rate_limit:"
// 	RedisThrottleKey = "throttle:"
// )

// func main() {
// 	// Initialize Redis and Database
// 	redisClient = initRedis()
// 	defer redisClient.Close()

// 	var err error
// 	// MySQL DSN: username:password@tcp(host:port)/dbname
// 	dsn := "naren:Python#123@tcp(localhost:3306)/players?charset=utf8mb4&parseTime=True&loc=Local"
// 	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
// 	if err != nil {
// 		log.Fatal("failed to connect database")
// 	}
// 	db.AutoMigrate(&Player{})

// 	// Fiber instance and CORS middleware
// 	app := fiber.New()
// 	app.Use(cors.New())

// 	// Middleware for rate limiting
// 	app.Use(rateLimitMiddleware)
// 	app.Use(throttleMiddleware)

// 	// Define routes
// 	app.Get("/players", cache("players", 10*time.Minute, getPlayers))
// 	app.Get("/players/:id", func(c *fiber.Ctx) error {
// 		id := c.Params("id")
// 		return cache("player:"+id, 10*time.Minute, getPlayerByID)(c)
// 	})

// 	// Start server
// 	log.Fatal(app.Listen(":3000"))
// }

// // Rate limiting middleware
// func rateLimitMiddleware(c *fiber.Ctx) error {
// 	clientIP := c.IP()
// 	key := RedisRateLimit + clientIP

// 	val, err := redisClient.Get(ctx, key).Int()
// 	if err == redis.Nil {
// 		redisClient.Set(ctx, key, 1, time.Duration(RATE_PERIOD)*time.Second)
// 	} else if err == nil {
// 		if val >= RATE_LIMIT {
// 			return fiber.NewError(fiber.StatusTooManyRequests, "Rate limit exceeded")
// 		}
// 		redisClient.Incr(ctx, key)
// 	} else {
// 		return err
// 	}
// 	return c.Next()
// }

// // Throttle middleware
// func throttleMiddleware(c *fiber.Ctx) error {
// 	clientIP := c.IP()
// 	key := RedisThrottleKey + clientIP

// 	data, err := redisClient.Get(ctx, key).Result()
// 	if err == redis.Nil {
// 		resetTime := time.Now().Add(time.Second * THROTTLE_PERIOD).Unix()
// 		redisClient.Set(ctx, key, fmt.Sprintf("1:%d", resetTime), time.Duration(THROTTLE_PERIOD)*time.Second)
// 	} else if err == nil {
// 		count, resetTimeStr, _ := parseThrottleData(data)
// 		resetTime, _ := strconv.ParseInt(resetTimeStr, 10, 64)

// 		if time.Now().Unix() < resetTime && count >= THROTTLE_LIMIT {
// 			return fiber.NewError(fiber.StatusTooManyRequests, "Throttle limit exceeded")
// 		}

// 		if time.Now().Unix() >= resetTime {
// 			count = 0
// 			resetTime = time.Now().Add(time.Second * COOLDOWN_PERIOD).Unix()
// 		}

// 		count++
// 		redisClient.Set(ctx, key, fmt.Sprintf("%d:%d", count, resetTime), time.Duration(THROTTLE_PERIOD)*time.Second)
// 	} else {
// 		return err
// 	}
// 	return c.Next()
// }

// // Helper function to parse throttle data
// func parseThrottleData(data string) (count int, resetTimeStr string, err error) {
// 	parts := strings.Split(data, ":")
// 	if len(parts) != 2 {
// 		err = fmt.Errorf("invalid throttle data format")
// 		return
// 	}
// 	count, err = strconv.Atoi(parts[0])
// 	if err != nil {
// 		return
// 	}
// 	resetTimeStr = parts[1]
// 	return
// }

// // Cache middleware - Example caching function
// func cache(key string, expiration time.Duration, next fiber.Handler) fiber.Handler {
// 	return func(c *fiber.Ctx) error {
// 		val, err := redisClient.Get(ctx, key).Bytes()
// 		if err == nil && len(val) > 0 {
// 			c.Response().SetBody(val)
// 			return nil
// 		}
// 		if err == redis.Nil {
// 			if err := next(c); err != nil {
// 				return err
// 			}
// 			redisClient.Set(ctx, key, c.Response().Body(), expiration)
// 		}
// 		return nil
// 	}
// }

// // Handler to get all players with cache
// func getPlayers(c *fiber.Ctx) error {
// 	var players []Player
// 	db.Find(&players)
// 	return c.JSON(players)
// }

// // Handler to get player by ID with cache
// func getPlayerByID(c *fiber.Ctx) error {
// 	id := c.Params("id")
// 	var player Player
// 	if err := db.First(&player, "playerID = ?", id).Error; err != nil {
// 		return fiber.NewError(fiber.StatusNotFound, "Player not found")
// 	}
// 	return c.JSON(player)
// }
