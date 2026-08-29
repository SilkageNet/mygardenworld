package apiserver

import "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"

// Handlers is the transport boundary for the public API. Each generated
// service is mounted through its own wrapper so route assembly cannot
// accidentally treat the entire application core as one giant handler.
type Handlers struct {
	Auth       *AuthHandler
	Account    *AccountHandler
	Automation *AutomationHandler
	Policy     *PolicyHandler
	Admin      *AdminHandler
	Redeem     *RedeemHandler
}

type AuthHandler struct{ *Services }
type AccountHandler struct{ *Services }
type AutomationHandler struct{ *Services }
type PolicyHandler struct{ *Services }
type AdminHandler struct{ *Services }
type RedeemHandler struct{ *Services }

func NewHandlers(services *Services) Handlers {
	return Handlers{
		Auth:       &AuthHandler{Services: services},
		Account:    &AccountHandler{Services: services},
		Automation: &AutomationHandler{Services: services},
		Policy:     &PolicyHandler{Services: services},
		Admin:      &AdminHandler{Services: services},
		Redeem:     &RedeemHandler{Services: services},
	}
}

var (
	_ mygardenworldv1connect.AuthServiceHandler           = (*AuthHandler)(nil)
	_ mygardenworldv1connect.AccountServiceHandler        = (*AccountHandler)(nil)
	_ mygardenworldv1connect.AutomationServiceHandler     = (*AutomationHandler)(nil)
	_ mygardenworldv1connect.PolicyServiceHandler         = (*PolicyHandler)(nil)
	_ mygardenworldv1connect.AdminServiceHandler          = (*AdminHandler)(nil)
	_ mygardenworldv1connect.RedeemExchangeServiceHandler = (*RedeemHandler)(nil)
)
