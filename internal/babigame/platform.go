package babigame

import (
	"fmt"
	"strings"
)

// Channel identifies the distribution / platform variant used by the adapter.
// Different channels can hit different host fronts, ship
// different version pinning, and require different device fingerprints
// (Android WeChat vs Alipay vs official APK vs iOS App Store, etc.).
//
// Channel values are kept in sync with the proto enum mygardenworld.v1.Channel.
// Persistence stores the channel as the lowercase name ("ios"); the gRPC
// surface uses the enum.
type Channel string

const (
	// ChannelUnspecified is the zero value; rejected at the API boundary.
	// Daemon code should never operate with this channel.
	ChannelUnspecified Channel = ""

	// ChannelIOS is the iOS channel currently supported by the adapter.
	ChannelIOS Channel = "ios"
)

// SupportedChannels lists every Channel the daemon can spin up. New channels
// get added here once their behavior is understood.
func SupportedChannels() []Channel {
	return []Channel{ChannelIOS}
}

// IsSupported reports whether c has a working ConfigForChannel mapping.
func IsSupported(c Channel) bool {
	for _, sc := range SupportedChannels() {
		if sc == c {
			return true
		}
	}
	return false
}

// ParseChannel canonicalises whatever the caller passed in. Accepts case
// variations and the proto enum string form ("CHANNEL_IOS").
func ParseChannel(s string) (Channel, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	v = strings.TrimPrefix(v, "channel_")
	c := Channel(v)
	if c == ChannelUnspecified {
		return ChannelUnspecified, fmt.Errorf("channel required (one of %v)", supportedChannelStrings())
	}
	if !IsSupported(c) {
		return ChannelUnspecified, fmt.Errorf("channel %q not supported (one of %v)", s, supportedChannelStrings())
	}
	return c, nil
}

func supportedChannelStrings() []string {
	xs := SupportedChannels()
	out := make([]string, len(xs))
	for i, c := range xs {
		out[i] = string(c)
	}
	return out
}

// ConfigForChannel returns the protocol Config tailored to the channel.
// Returns an error for unimplemented channels so the daemon refuses to
// invent values when a future channel lands without a code update.
func ConfigForChannel(c Channel) (Config, error) {
	switch c {
	case ChannelIOS:
		return iOSConfig(), nil
	case ChannelUnspecified:
		return Config{}, fmt.Errorf("channel required (one of %v)", supportedChannelStrings())
	default:
		return Config{}, fmt.Errorf("channel %q not yet implemented (capture needed)", c)
	}
}

// iOSConfig returns the iOS-specific protocol identifiers.
//
// When the app version bumps and new authorized observations are available,
// update the version-pinning fields here.
func iOSConfig() Config {
	return Config{
		AppID:             "95cdabc1e87532edb21c99f4ed845653",
		PackageName:       "cn.lbwdhysj.gf.ios",
		AppVersion:        "1.1.17",
		AppVersionCode:    "1001017",
		ClientVersion:     "360.0.24",
		GameVersion:       "360.0.24",
		RNVersion:         "v3.3.2.41",
		SDKVersion:        "7.0.4",
		MdGid:             160,
		ChannelID:         459,
		PackageID:         494,
		SDKID:             10000,
		Env:               "prod",
		Area:              "cn",
		ZoneCode:          "ys",
		GWXorMask:         0x77,
		GWSignKey:         "smallaitt",
		WSSentinel:        "$#|#$",
		HostAPI:           "api.babigame.cn",
		HostMOAC:          "moac.babigame.cn",
		HostAPIRP:         "apirp.babigame.cn",
		HostAPIPush:       "apipush.babigame.cn",
		HostGW:            "hygnhf2.babigame.cn",
		HostCDN:           "hygncdn.babigame.cn",
		HostCustomer:      "customsrevicesource.babigame.cn",
		HostWSDefault:     "hy2gnhf118.babigame.cn",
		HostWSDefaultPort: 54825,
		UserAgent:         "HuaYuann/2 CFNetwork/3860.600.12 Darwin/25.5.0",
		OSVersion:         "26.5",
		DeviceBrand:       "Apple",
		DeviceModel:       "iPhone 15 Pro",
		CPUType:           "CPU_TYPE_ARM64",
		RAMMB:             "7680",
		ScreenHeightPx:    "2556",
		ScreenWidthPx:     "1179",
		NetworkType:       "wifi",
		SysLanguage:       "zh-Hans-CN",
		RuntimeLanguage:   "zh",
		TimeZoneHour:      "8",
	}
}
