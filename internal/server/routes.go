package server

import (
	"fmt"
	"os"
	"runtime"
	"rytr/internal/database/dto"
	"rytr/internal/database/models"
	"rytr/internal/database/repositories"
	"rytr/internal/utils"
	"time"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func (s *FiberServer) RegisterFiberRoutes() {
	s.App.Post("/login", s.login)
	s.App.Post("/register", s.registerUser)
	s.App.Get("/health", s.healthHandler)
	// endpoint to monitor memory
	s.App.Get("/memory", func(c *fiber.Ctx) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memoryInfo := fmt.Sprintf("Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
			bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)
		return c.SendString(memoryInfo)
	})
	secret := os.Getenv("SECRET_KEY")
	s.App.Use(jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(secret)},
	}))

	s.App.Post("/reset-password", s.resetPassword)
	s.App.Get("/profile", s.getUserProfile)
	s.App.Put("/profile", s.updateUserProfile)

	s.App.Post("/cards", s.createCard)
	s.App.Get("/cards", s.getAllCards)
	s.App.Get("/cards/pending", s.getPendingCards)
	s.App.Get("/cards/:id<int />", s.getSingleCard)
	s.App.Put("/cards/:id<int />", s.updateCard)
	s.App.Put("/cards/status/:id<int />", s.updateCardStatus)
	s.App.Delete("/cards/:id<int />", s.deleteCard)

	s.App.Post("/notes", s.createNote)
	s.App.Get("/notes", s.getAllNotes)
	s.App.Get("/notes/:id", s.getSingleNote)
	s.App.Put("/notes/:id", s.updateNote)
	s.App.Delete("/notes/:id", s.deleteNote)

	s.App.Get(("/search"), s.searchData)

	s.App.Post("/gemini", s.geminiHandler)
}

func (s *FiberServer) healthHandler(c *fiber.Ctx) error {
	return c.JSON(s.db.Health())
}

func (s *FiberServer) login(c *fiber.Ctx) error {
	credentials := dto.LoginCredentials{}
	if err := c.BodyParser(&credentials); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid json body"})
	}
	repo := repositories.NewUserRepository(s.db.DB())
	user, err := repo.GetByEmail(c.Context(), credentials.Email)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "user not found"})
	}
	// Throws Unauthorized error
	if !utils.CheckPasswordHash(credentials.Password, user.Password) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Invalid credentials"})
	}

	// Create the Claims
	claims := jwt.MapClaims{
		"email": user.Email,
		"exp":   time.Now().Add(time.Hour * 72).Unix(),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	t, err := token.SignedString([]byte(os.Getenv("SECRET_KEY")))
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"token": t})
}

func (s *FiberServer) registerUser(c *fiber.Ctx) error {
	user := models.User{}

	if err := c.BodyParser(&user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid json body"})
	}
	var err error
	user.Password, err = utils.HashPassword(user.Password)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Error hashing password, max-length:64 characters"})
	}
	repo := repositories.NewUserRepository(s.db.DB())
	err = repo.Create(c.Context(), &user)
	if err != nil {
		return c.Status(fiber.StatusNotAcceptable).JSON(fiber.Map{"message": "User already exist"})

	}
	return c.JSON(fiber.Map{"message": "created user successfully"})

}

func (s *FiberServer) resetPassword(c *fiber.Ctx) error {
	// Get user from JWT token
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)

	// Get user from database
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	// Parse request body
	var req dto.PasswordResetRequest
	if err := c.BodyParser(&req); err != nil {
		fmt.Println(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request body"})
	}
	// Validate request data
	if req.OldPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Old password and new password are required"})
	}

	// Call repository to reset password
	err = userRepo.ResetPassword(c.Context(), currentUser.ID, req.OldPassword, req.NewPassword)
	if err != nil {
		if err.Error() == "user not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
		}
		if err.Error() == "incorrect password" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Incorrect password"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to reset password"})
	}

	return c.JSON(fiber.Map{"message": "Password reset successful"})
}

func (s *FiberServer) getUserProfile(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)

	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}

	return c.JSON(fiber.Map{
		"id":         currentUser.ID,
		"email":      currentUser.Email,
		"first_name": currentUser.FirstName,
		"last_name":  currentUser.LastName,
		"created_at": currentUser.CreatedAt,
		"updated_at": currentUser.UpdatedAt,
	})
}

func (s *FiberServer) updateUserProfile(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)

	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}

	var req dto.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request body"})
	}

	if req.FirstName == "" || req.LastName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "First name and last name are required"})
	}

	currentUser.FirstName = req.FirstName
	currentUser.LastName = req.LastName

	err = userRepo.Update(c.Context(), currentUser)
	if err != nil {
		if err.Error() == "user not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to update profile"})
	}

	return c.JSON(fiber.Map{
		"message": "Profile updated successfully",
		"user": fiber.Map{
			"id":         currentUser.ID,
			"email":      currentUser.Email,
			"first_name": currentUser.FirstName,
			"last_name":  currentUser.LastName,
			"created_at": currentUser.CreatedAt,
			"updated_at": currentUser.UpdatedAt,
		},
	})
}

// Card endpoints

func (s *FiberServer) createCard(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	cardRepo := repositories.NewCardRepository(s.db.DB())
	card := models.Card{}
	if err := c.BodyParser(&card); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid json"})

	}
	card.UserID = currentUser.ID
	if err := cardRepo.Create(c.Context(), &card); err != nil {
		return c.Status(fiber.StatusNotAcceptable).JSON(fiber.Map{"message": "This Card already exists"})
	}
	return c.JSON(fiber.Map{"message": "Card added successfully"})
}

func (s *FiberServer) getSingleCard(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	id := c.Params("id")
	cardRepo := repositories.NewCardRepository(s.db.DB())
	uid, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid uid"})
	}
	card, err := cardRepo.GetByID(c.Context(), uid, currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Card not found"})

	}
	return c.JSON(fiber.Map{"card": card})
}

func (s *FiberServer) getAllCards(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	cardRepo := repositories.NewCardRepository(s.db.DB())
	cards, err := cardRepo.GetAll(c.Context(), currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to fetch cards"})
	}
	return c.JSON(fiber.Map{"cards": cards})
}

func (s *FiberServer) getPendingCards(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	cardRepo := repositories.NewCardRepository(s.db.DB())
	cards, err := cardRepo.GetPending(c.Context(), currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "unable to fetch cards"})
	}
	return c.JSON(fiber.Map{"cards": cards})
}

func (s *FiberServer) updateCard(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	id := c.Params("id")
	cardRepo := repositories.NewCardRepository(s.db.DB())
	var card = models.Card{}
	if err := c.BodyParser(&card); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	card.ID, err = uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"message": "invalid uid"})
	}
	err = cardRepo.Update(c.Context(), &card, currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "card updated successfully",
	})
}

func (s *FiberServer) updateCardStatus(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	id := c.Params("id")
	cardRepo := repositories.NewCardRepository(s.db.DB())
	var status dto.CardStatus
	if err := c.BodyParser(&status); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"message": "invalid uid"})
	}
	err = cardRepo.UpdateStatus(c.Context(), uid, status.Status, currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "card updated successfully",
	})
}

func (s *FiberServer) deleteCard(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	id := c.Params("id")
	uid, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"message": "invalid uid"})
	}
	cardRepo := repositories.NewCardRepository(s.db.DB())
	err = cardRepo.Delete(c.Context(), uid, currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "card deleted successfully",
	})
}

// Notes endpoints

func (s *FiberServer) createNote(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	noteRepo := repositories.NewNoteRepository(s.db.DB())
	note := models.Note{}
	if err := c.BodyParser(&note); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	note.UserID = currentUser.ID
	if err := noteRepo.Create(c.Context(), &note); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Note added successfully"})
}

func (s *FiberServer) getSingleNote(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	id := c.Params("id")
	noteRepo := repositories.NewNoteRepository(s.db.DB())
	uid, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"message": "invalid uid"})
	}
	note, err := noteRepo.GetByID(c.Context(), uid, currentUser.ID)
	if err != nil {
		return c.JSON(fiber.Map{"note": nil})
	}
	return c.JSON(fiber.Map{"note": note})
}

func (s *FiberServer) getAllNotes(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	limit := c.QueryInt("limit")
	noteRepo := repositories.NewNoteRepository(s.db.DB())
	notes, err := noteRepo.GetAll(c.Context(), currentUser.ID, limit)
	if err != nil {
		c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to fetch user notes"})
	}
	return c.JSON(fiber.Map{"notes": notes, "user": currentUser})
}

func (s *FiberServer) updateNote(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	id := c.Params("id")
	noteRepo := repositories.NewNoteRepository(s.db.DB())
	var note = models.Note{}

	if err := c.BodyParser(&note); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"message": "invalid uid"})
	}
	note.ID = uid
	fmt.Println("data:\t title:", note.Title, "\tcontent:", note.Content)
	err = noteRepo.Update(c.Context(), &note, currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "note updated successfully",
	})
}

func (s *FiberServer) deleteNote(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	id := c.Params("id")
	uid, err := uuid.Parse(id)
	if err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{"message": "invalid uid"})
	}
	noteRepo := repositories.NewNoteRepository(s.db.DB())
	err = noteRepo.Delete(c.Context(), uid, currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "note deleted successfully",
	})
}

func (s *FiberServer) searchData(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	currentUser, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	searchRepo := repositories.NewSearchRepository(s.db.DB())
	data, err := searchRepo.SearchQuery(c.Context(), c.Query("q"), currentUser.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to fetch cards"})
	}
	return c.JSON(fiber.Map{"results": data})
}

func (s *FiberServer) geminiHandler(c *fiber.Ctx) error {
	user := c.Locals("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	email := claims["email"].(string)
	userRepo := repositories.NewUserRepository(s.db.DB())
	_, err := userRepo.GetByEmail(c.Context(), email)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid user"})
	}
	var req dto.GeminiRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request body"})
	}

	prompt := req.Prompt
	var config *genai.GenerateContentConfig = &genai.GenerateContentConfig{Temperature: genai.Ptr[float32](0.5)}
	response, err := s.geminiClient.Models.GenerateContent(
		c.Context(),
		"models/gemini-2.0-flash",
		genai.Text(prompt),
		config,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Gemini API error", "error": err.Error()})
	}
	if len(response.Candidates) > 0 && len(response.Candidates[0].Content.Parts) > 0 {
		text, _ := response.Text()
		return c.JSON(fiber.Map{"response": text})
	}

	return c.JSON(fiber.Map{"response": "No response from Gemini API"})
}
