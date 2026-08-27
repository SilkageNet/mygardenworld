package apiserver

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestPublicProtocolIsACleanBreakingBaseline(t *testing.T) {
	t.Helper()
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != "mygardenworld.v1" {
			return true
		}
		for index := 0; index < file.Messages().Len(); index++ {
			assertDenseMessageDescriptor(t, file.Messages().Get(index))
		}
		for index := 0; index < file.Enums().Len(); index++ {
			assertDenseEnumDescriptor(t, file.Enums().Get(index))
		}
		return true
	})
}

func assertDenseMessageDescriptor(t *testing.T, message protoreflect.MessageDescriptor) {
	t.Helper()
	if message.ReservedNames().Len() != 0 || message.ReservedRanges().Len() != 0 {
		t.Errorf("%s retains compatibility reservations", message.FullName())
	}
	fields := message.Fields()
	for number := 1; number <= fields.Len(); number++ {
		if fields.ByNumber(protoreflect.FieldNumber(number)) == nil {
			t.Errorf("%s has a field-number gap at %d", message.FullName(), number)
		}
	}
	for index := 0; index < message.Messages().Len(); index++ {
		assertDenseMessageDescriptor(t, message.Messages().Get(index))
	}
	for index := 0; index < message.Enums().Len(); index++ {
		assertDenseEnumDescriptor(t, message.Enums().Get(index))
	}
}

func assertDenseEnumDescriptor(t *testing.T, enum protoreflect.EnumDescriptor) {
	t.Helper()
	if enum.ReservedNames().Len() != 0 || enum.ReservedRanges().Len() != 0 {
		t.Errorf("%s retains compatibility reservations", enum.FullName())
	}
	values := enum.Values()
	for number := 0; number < values.Len(); number++ {
		if values.ByNumber(protoreflect.EnumNumber(number)) == nil {
			t.Errorf("%s has an enum-number gap at %d", enum.FullName(), number)
		}
	}
}
