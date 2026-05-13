package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"
)

type InquiryRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

func corsAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	if raw == "" {
		return []string{
			"http://localhost:5173",
			"https://dabargroup.com",
			"https://www.dabargroup.com",
		}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{
			"http://localhost:5173",
			"https://dabargroup.com",
			"https://www.dabargroup.com",
		}
	}
	return out
}

func main() {
	if os.Getenv("RENDER") == "true" || strings.EqualFold(os.Getenv("GIN_MODE"), "release") {
		gin.SetMode(gin.ReleaseMode)
	}

	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins: corsAllowedOrigins(),
		AllowMethods: []string{
			"GET",
			"POST",
			"OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
		},
	}))

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Dabar Group Backend Running",
		})
	})

	router.POST("/send-inquiry", SendInquiryHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("Server running on port:", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func SendInquiryHandler(c *gin.Context) {
	var req InquiryRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := 465
	if v := strings.TrimSpace(os.Getenv("SMTP_PORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			smtpPort = n
		}
	}
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	if smtpHost == "" || smtpUser == "" || smtpPass == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "SMTP is not configured",
		})
		return
	}

	body := fmt.Sprintf(`
		<h2>New Inquiry Received</h2>

		<p><strong>Name:</strong> %s</p>
		<p><strong>Email:</strong> %s</p>
		<p><strong>Phone:</strong> %s</p>
		<p><strong>Subject:</strong> %s</p>
		<p><strong>Message:</strong> %s</p>
	`,
		req.Name,
		req.Email,
		req.Phone,
		req.Subject,
		req.Message,
	)

	m := gomail.NewMessage()

	m.SetHeader("From", smtpUser)
	m.SetHeader("To", smtpUser)
	m.SetHeader("Reply-To", req.Email)
	m.SetHeader("Subject", "New Inquiry - "+req.Subject)

	m.SetBody("text/html", body)

	d := gomail.NewDialer(
		smtpHost,
		smtpPort,
		smtpUser,
		smtpPass,
	)

	// Port 465 typically uses implicit TLS (SSL). Port 587 uses STARTTLS (SSL=false in gomail).
	ssl := smtpPort == 465
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_SSL"))) {
	case "true", "1", "yes":
		ssl = true
	case "false", "0", "no":
		ssl = false
	}
	d.SSL = ssl

	if err := d.DialAndSend(m); err != nil {
		log.Printf("smtp send: %v", err)
		payload := gin.H{
			"success": false,
			"message": "Failed to send email",
		}
		if gin.Mode() != gin.ReleaseMode {
			payload["error"] = err.Error()
		}
		c.JSON(http.StatusInternalServerError, payload)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Inquiry sent successfully",
	})
}
