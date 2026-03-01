package rfid

import (
	"github.com/gofiber/fiber/v2"
	"parkieee/pkg/middleware"
	"parkieee/pkg/validator"
)

func RegisterRoutes(router fiber.Router, svc ServicePort, auth middleware.TokenValidator, v *validator.Validator) {
	adapter := newHTTPAdapter(svc, v)

	cards := router.Group("/rfid/cards", middleware.Auth(auth))

	cards.Get("/", adapter.h.listCards)
	cards.Get("/uid/:uid", adapter.h.getCardByUID)
	cards.Get("/:id", adapter.h.getCard)

	// register or get — operator dan ke atas semua bisa (dipakai saat tap pertama)
	cards.Post("/", adapter.h.registerOrGet)

	// link vehicle dan deactivate — butuh permission rfid.manage
	manage := cards.Group("", middleware.RequirePermission("rfid.manage"))
	manage.Patch("/:id/link-vehicle", adapter.h.linkVehicle)
	manage.Delete("/:id", adapter.h.deactivate)
}
