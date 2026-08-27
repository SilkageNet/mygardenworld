package apiserver

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestPublicProtocolDeclaresNoReservedNamesOrNumbers(t *testing.T) {
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != "mygardenworld.v1" {
			return true
		}
		checkMessageReservations(t, string(file.Path()), file.Messages())
		checkEnumReservations(t, string(file.Path()), file.Enums())
		return true
	})
}

func checkMessageReservations(t *testing.T, path string, messages protoreflect.MessageDescriptors) {
	t.Helper()
	for index := 0; index < messages.Len(); index++ {
		message := messages.Get(index)
		if message.ReservedNames().Len() > 0 || message.ReservedRanges().Len() > 0 {
			t.Errorf("%s %s contains reserved declarations", path, message.FullName())
		}
		checkMessageReservations(t, path, message.Messages())
		checkEnumReservations(t, path, message.Enums())
	}
}

func checkEnumReservations(t *testing.T, path string, enums protoreflect.EnumDescriptors) {
	t.Helper()
	for index := 0; index < enums.Len(); index++ {
		enum := enums.Get(index)
		if enum.ReservedNames().Len() > 0 || enum.ReservedRanges().Len() > 0 {
			t.Errorf("%s %s contains reserved declarations", path, enum.FullName())
		}
		if strings.Contains(string(enum.FullName()), "V2") {
			t.Errorf("%s %s exposes a second protocol generation", path, enum.FullName())
		}
	}
}
