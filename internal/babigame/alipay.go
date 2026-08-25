package babigame

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAlipayGatewayURL = "https://webgwmobiler.alipay.com"
	defaultAlipayYXTURL     = "https://h5sdk.hnycgames.cn"
	alipayWebGWAppID        = "180020010001270314"
	alipayGameSlug          = "xjskp"
	alipayWebTokenHeader    = "x-game-token-pcweb"
	alipayDefaultUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/140.0.0.0 Safari/537.36"
)

var ErrAlipayQRExpired = errors.New("alipay qr code expired")

// AlipayQRChallenge is the short-lived authorization request rendered by the
// local Web UI. Token is kept server-side; only URL is sent to the QR renderer.
type AlipayQRChallenge struct {
	Token string
	URL   string
}

// AlipayWebGrant is returned once the phone authorizes the PC game-center
// session. Token is a reusable channel credential and must be encrypted at rest.
type AlipayWebGrant struct {
	Token  string
	UserID string
}

// AlipayYXTGrant is the channel SDK assertion accepted by Babi /game/login.
// Values deliberately retain the JSON types returned by YXT. Its signature
// covers these fields, so coercing a numeric login time to a string makes an
// otherwise valid assertion fail verification.
type AlipayYXTGrant struct {
	Game          any
	Channel       any
	UserID        any
	LoginTime     any
	Sign          any
	ChannelUserID any
}

// AlipayAuthProvider is intentionally small so the API coordinator can be
// tested without contacting Alipay or the game backend.
type AlipayAuthProvider interface {
	BeginQR(context.Context) (AlipayQRChallenge, error)
	PollQR(context.Context, string) (AlipayWebGrant, bool, error)
	LoginWithWebGrant(context.Context, *HTTPClient, AlipayWebGrant) (*Session, error)
}

// AlipayClient implements the observed wanyiwan PC login bridge:
// QR -> PC web token -> authCode -> yxt assertion -> Babi common login chain.
type AlipayClient struct {
	Cfg        Config
	GatewayURL string
	YXTURL     string
	HTTPClient *http.Client
	UserAgent  string

	mu     sync.Mutex
	ctoken string
}

func NewAlipayClient(cfg Config) *AlipayClient {
	return &AlipayClient{
		Cfg:        cfg,
		GatewayURL: defaultAlipayGatewayURL,
		YXTURL:     defaultAlipayYXTURL,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
		UserAgent:  alipayDefaultUserAgent,
	}
}

func (c *AlipayClient) BeginQR(ctx context.Context) (AlipayQRChallenge, error) {
	resp, err := c.postGateway(ctx, "/gameauth/com.alipay.gameauth.common.facade.service.GameCenterPcAuthFacade/getLoginToken", map[string]any{}, "", "")
	if err != nil {
		return AlipayQRChallenge{}, err
	}
	data := mapOf(resp["data"])
	qr := mapOf(data["qrCode"])
	challenge := AlipayQRChallenge{Token: stringOf(qr["token"]), URL: stringOf(qr["url"])}
	if challenge.Token == "" || challenge.URL == "" {
		return AlipayQRChallenge{}, errors.New("alipay getLoginToken response missing qrCode token/url")
	}
	return challenge, nil
}

// PollQR returns (grant, authorized, error). A successful HTTP response with
// no token is a normal waiting state, not an error.
func (c *AlipayClient) PollQR(ctx context.Context, qrToken string) (AlipayWebGrant, bool, error) {
	if strings.TrimSpace(qrToken) == "" {
		return AlipayWebGrant{}, false, errors.New("alipay qr token required")
	}
	resp, err := c.postGateway(ctx, "/gameauth/com.alipay.gameauth.common.facade.service.GameCenterPcAuthFacade/loginForPc", map[string]any{
		"token":     qrToken,
		"userAgent": c.userAgent(),
	}, "", "")
	if err != nil {
		var upstream *alipayBusinessError
		if errors.As(err, &upstream) {
			if upstream.tokenInvalid() {
				return AlipayWebGrant{}, false, ErrAlipayQRExpired
			}
			// loginForPc uses business errors as its ordinary "not scanned yet"
			// response. The official page keeps polling every second unless the
			// QR token itself is invalid.
			return AlipayWebGrant{}, false, nil
		}
		return AlipayWebGrant{}, false, err
	}
	data := mapOf(resp["data"])
	grant := AlipayWebGrant{Token: stringOf(data["token"]), UserID: stringOf(data["userId"])}
	if grant.Token == "" {
		return AlipayWebGrant{}, false, nil
	}
	if grant.UserID == "" {
		return AlipayWebGrant{}, false, errors.New("alipay loginForPc response missing userId")
	}
	return grant, true, nil
}

func (c *AlipayClient) LoginWithWebGrant(ctx context.Context, gameHTTP *HTTPClient, grant AlipayWebGrant) (*Session, error) {
	if gameHTTP == nil {
		return nil, errors.New("alipay game HTTP client required")
	}
	gameAppID, err := c.queryGameAppID(ctx, grant)
	if err != nil {
		return nil, err
	}
	authCode, err := c.queryGameAuthCode(ctx, grant, gameAppID)
	if err != nil {
		return nil, err
	}
	yxt, err := c.exchangeYXT(ctx, authCode)
	if err != nil {
		return nil, err
	}
	payload, err := c.gameLoginPayload(gameHTTP, yxt)
	if err != nil {
		return nil, err
	}
	gameLogin, err := gameHTTP.GameLoginWithPayload(ctx, payload)
	if err != nil {
		return nil, err
	}
	return FinishLoginWithGameLogin(ctx, gameHTTP, NativeLogin{}, gameLogin, gameHTTP.Cfg.IsSimulator)
}

func (c *AlipayClient) queryGameAppID(ctx context.Context, grant AlipayWebGrant) (string, error) {
	if strings.TrimSpace(grant.Token) == "" || strings.TrimSpace(grant.UserID) == "" {
		return "", errors.New("alipay web grant requires token and userId")
	}
	resp, err := c.postGateway(ctx, "/gamecenterhome/com.alipay.gamecenterhome.common.facade.service.GameCenterPcGameFacade/queryPcGameInfo/uprodhatchstation66500008", map[string]any{
		"gameId": alipayGameSlug,
	}, grant.Token, grant.UserID)
	if err != nil {
		return "", fmt.Errorf("alipay query game info: %w", err)
	}
	appID := stringOf(mapOf(resp["data"])["appId"])
	if appID == "" {
		return "", errors.New("alipay queryPcGameInfo response missing appId")
	}
	return appID, nil
}

func (c *AlipayClient) queryGameAuthCode(ctx context.Context, grant AlipayWebGrant, gameAppID string) (string, error) {
	if strings.TrimSpace(grant.Token) == "" || strings.TrimSpace(grant.UserID) == "" {
		return "", errors.New("alipay web grant requires token and userId")
	}
	if strings.TrimSpace(gameAppID) == "" {
		return "", errors.New("alipay game appId required")
	}
	resp, err := c.postGateway(ctx, "/gamecenterhome/com.alipay.gamecenterhome.common.facade.service.GameCenterPcGameFacade/queryPcGameAuthInfo/uprodhatchstation66500008", map[string]any{
		"appId": gameAppID,
	}, grant.Token, grant.UserID)
	if err != nil {
		return "", fmt.Errorf("alipay query game auth: %w", err)
	}
	authCode := stringOf(mapOf(resp["data"])["authCode"])
	if authCode == "" {
		return "", errors.New("alipay queryPcGameAuthInfo response missing authCode")
	}
	return authCode, nil
}

func (c *AlipayClient) exchangeYXT(ctx context.Context, authCode string) (AlipayYXTGrant, error) {
	form := url.Values{"authCode": {authCode}, "scene": {"other"}}
	endpoint := strings.TrimRight(c.YXTURL, "/") + "/Channel/login/yxtGame/wdhysj/yxtChannel/myxyx/yxtSubChannel/myxyx"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return AlipayYXTGrant{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent())
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return AlipayYXTGrant{}, fmt.Errorf("alipay yxt login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return AlipayYXTGrant{}, fmt.Errorf("alipay yxt login read: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return AlipayYXTGrant{}, fmt.Errorf("alipay yxt login HTTP %d", resp.StatusCode)
	}
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return AlipayYXTGrant{}, errors.New("alipay yxt login returned invalid JSON")
	}
	if readInt(envelope, "errorCode") != 0 {
		return AlipayYXTGrant{}, fmt.Errorf("alipay yxt login rejected: code=%d", readInt(envelope, "errorCode"))
	}
	data := mapOf(envelope["data"])
	grant := AlipayYXTGrant{
		Game:          data["yxtGame"],
		Channel:       data["yxtChannel"],
		UserID:        data["yxtUserId"],
		LoginTime:     data["yxtLoginTime"],
		Sign:          data["yxtSign"],
		ChannelUserID: data["yxtChannelUserId"],
	}
	if stringOf(grant.Game) == "" {
		grant.Game = "wdhysj"
	}
	if stringOf(grant.Channel) == "" {
		grant.Channel = "myxyx"
	}
	if stringOf(grant.UserID) == "" || stringOf(grant.LoginTime) == "" || stringOf(grant.Sign) == "" || stringOf(grant.ChannelUserID) == "" {
		return AlipayYXTGrant{}, errors.New("alipay yxt login response missing signed user fields")
	}
	return grant, nil
}

func (c *AlipayClient) gameLoginPayload(gameHTTP *HTTPClient, yxt AlipayYXTGrant) (map[string]any, error) {
	launch := map[string]any{
		"query": map[string]any{"channel": "other"},
		"scene": "1000",
	}
	openData, err := json.Marshal(launch)
	if err != nil {
		return nil, err
	}
	systemInfo, err := json.Marshal(map[string]any{
		"platform":     gameHTTP.Cfg.SDKPlatform,
		"version":      gameHTTP.Cfg.ClientVersion,
		"model":        gameHTTP.Cfg.DeviceModel,
		"brand":        gameHTTP.Cfg.DeviceBrand,
		"screenHeight": gameHTTP.Cfg.ScreenHeightPx,
		"screenWidth":  gameHTTP.Cfg.ScreenWidthPx,
		"system":       gameHTTP.Cfg.OSVersion,
		"language":     gameHTTP.Cfg.SysLanguage,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"open_data":        string(openData),
		"scene":            "1000",
		"system_info":      string(systemInfo),
		"yxtGame":          yxt.Game,
		"yxtChannel":       yxt.Channel,
		"yxtUserId":        yxt.UserID,
		"yxtLoginTime":     yxt.LoginTime,
		"yxtSign":          yxt.Sign,
		"yxtChannelUserId": yxt.ChannelUserID,
	}, nil
}

func (c *AlipayClient) postGateway(ctx context.Context, path string, payload map[string]any, webToken, userID string) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(strings.TrimRight(c.GatewayURL, "/") + path)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("ctoken", c.csrfToken())
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("x-webgw-appId", alipayWebGWAppID)
	req.Header.Set("x-webgw-version", "2.0")
	req.Header.Set("x-webgw-ldc-uid", alipayLDCUID(userID))
	if webToken != "" {
		req.Header.Set(alipayWebTokenHeader, webToken)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("alipay gateway %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("alipay gateway %s read: %w", path, err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("alipay gateway %s HTTP %d", path, resp.StatusCode)
	}
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("alipay gateway %s returned invalid JSON", path)
	}
	if success, present := envelope["success"].(bool); present && !success {
		return nil, &alipayBusinessError{
			Code:    stringOf(firstNonEmpty(envelope, "errorCode", "resultCode")),
			Message: SafeUTF8(stringOf(firstNonEmpty(envelope, "errorMsg", "errorMessage", "resultMessage"))),
		}
	}
	return envelope, nil
}

// alipayLDCUID mirrors the PC page request interceptor. Once signed in it
// routes on the two characters immediately before the final user-ID digit;
// pre-login calls use an arbitrary two-digit shard.
func alipayLDCUID(userID string) string {
	userID = strings.TrimSpace(userID)
	if len(userID) >= 2 {
		start := len(userID) - 3
		if start < 0 {
			start = 0
		}
		if end := len(userID) - 1; end > start {
			return userID[start:end]
		}
	}
	var randomByte [1]byte
	if _, err := cryptorand.Read(randomByte[:]); err == nil {
		return strconv.Itoa(int(randomByte[0]) % 100)
	}
	return strconv.Itoa(int(time.Now().UnixNano() % 100))
}

func (c *AlipayClient) csrfToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctoken == "" {
		c.ctoken = "bigfish_ctoken_" + strconv.FormatInt(time.Now().UnixMilli(), 22)
	}
	return c.ctoken
}

func (c *AlipayClient) userAgent() string {
	if c.UserAgent != "" {
		return c.UserAgent
	}
	return alipayDefaultUserAgent
}

func (c *AlipayClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

type alipayBusinessError struct {
	Code    string
	Message string
}

func (e *alipayBusinessError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("alipay gateway rejected request: code=%s", e.Code)
	}
	return fmt.Sprintf("alipay gateway rejected request: code=%s message=%s", e.Code, e.Message)
}

func (e *alipayBusinessError) tokenInvalid() bool {
	value := strings.ToUpper(e.Code + " " + e.Message)
	return strings.Contains(value, "TOKEN_INVALID") || strings.Contains(value, "TOKEN_EXPIRED") || strings.Contains(value, "二维码已过期")
}
