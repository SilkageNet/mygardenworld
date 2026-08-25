package babigame

// Config holds the version-specific identifiers and the host map. Override
// fields when the game version bumps; everything that varies is here.
type Config struct {
	// App identity
	AppID          string
	PackageName    string
	AppVersion     string // 1.1.12
	AppVersionCode string // 1001012
	ClientVersion  string // 330.0.31
	GameVersion    string // 330.0.31
	RNVersion      string // v3.3.2.32
	SDKVersion     string // 7.0.4
	SDKPlatform    string // iOS / Browser
	MobilePlatform string // ios / browser
	GamePlatform   string // mobilegame / minigame
	OSType         int    // protocol enum used by index.login
	IsNative       bool
	IsSimulator    int
	DeviceType     string // Phone / PC

	// Channel / project
	MdGid     int    // 160
	ChannelID int    // 459
	PackageID int    // 494
	SDKID     int    // 10000
	Env       string // prod
	Area      string // cn
	ZoneCode  string // ys

	// /gw + WS crypto
	GWXorMask  byte   // 0x77
	GWSignKey  string // smallaitt
	WSSentinel string // $#|#$

	// Hosts
	HostAPI      string
	HostMOAC     string
	HostAPIRP    string
	HostAPIPush  string
	HostGW       string
	HostCDN      string
	HostCustomer string

	// Default WS host - only used as a fallback when getGsInfoList returns no
	// candidates. Real WS host is discovered at login time.
	HostWSDefault     string
	HostWSDefaultPort int // 54825 (port_ssl)

	// Channel-specific device fingerprint template.
	UserAgent       string
	OSVersion       string
	DeviceBrand     string
	DeviceModel     string
	CPUType         string
	RAMMB           string
	ScreenHeightPx  string
	ScreenWidthPx   string
	NetworkType     string
	SysLanguage     string
	RuntimeLanguage string
	TimeZoneHour    string
}
