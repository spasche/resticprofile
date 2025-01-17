package config

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"reflect"
	"strings"
	"testing"

	"github.com/creativeprojects/resticprofile/constants"
	"github.com/creativeprojects/resticprofile/restic"
	"github.com/creativeprojects/resticprofile/util/collect"
	"github.com/stretchr/testify/assert"
)

func customizeProperty(sectionName string, info PropertyInfo) PropertyInfo {
	props := map[string]PropertyInfo{info.Name(): info}
	customizeProperties(sectionName, props)
	return props[info.Name()]
}

func TestResticPropertyDescriptionFilter(t *testing.T) {
	tests := []struct {
		original, expected string
	}{
		{
			original: `be verbose (specify multiple times or a level using --verbose=n, max level/times is 4)`,
			expected: `be verbose (true for level 1 or a number for increased verbosity, max level is 4)`,
		},
		{
			original: `snapshot id to search in (can be given multiple times)`,
			expected: `snapshot id to search in`,
		},
		{
			original: `set extended option (key=value, can be specified multiple times)`,
			expected: `set extended option (key=value)`,
		},
		{
			original: `add tags for the new snapshot in the format tag[,tag,...] (can be specified multiple times).`,
			expected: `add tags for the new snapshot in the format tag[,tag,...].`,
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			info := customizeProperty("",
				newResticPropertyInfo("any", restic.Option{Description: test.original}))
			assert.Equal(t, test.expected, info.Description())
		})
	}
}

func TestGlobalDefaultCommandProperty(t *testing.T) {
	info := NewGlobalInfo().PropertyInfo("default-command")
	require.NotNil(t, info)
	assert.ElementsMatch(t, restic.CommandNamesForVersion(restic.AnyVersion), info.ExampleValues())
}

func TestResticVerboseProperty(t *testing.T) {
	orig := newResticPropertyInfo("verbose", restic.Option{})
	assert.True(t, orig.CanBeString())
	assert.False(t, orig.CanBeBool())
	assert.False(t, orig.CanBeNumeric())
	assert.False(t, orig.MustBeInteger())

	customizeProperty("", orig)
	assert.True(t, orig.CanBeString())
	assert.True(t, orig.CanBeBool())
	assert.True(t, orig.CanBeNumeric())
	assert.True(t, orig.MustBeInteger())
}

func TestHostTagPathProperty(t *testing.T) {
	examples := []string{"true", "false", `"{{property}}"`}
	note := `Boolean true is replaced with the {{property}}s from section "backup".`
	hostNote := `Boolean true is replaced with the hostname of the system.`
	backupNote := `Boolean true is unsupported in section "backup".`

	tests := []struct {
		section, property, note, format string
		examples                        []string
	}{
		{section: "any", property: constants.ParameterHost, note: hostNote, format: "hostname"},
		{section: "any", property: constants.ParameterPath},
		{section: "any", property: constants.ParameterTag},

		{section: constants.CommandBackup, property: constants.ParameterHost, note: hostNote, format: "hostname"},
		{section: constants.CommandBackup, property: constants.ParameterPath, note: backupNote, examples: []string{"false", `"{{property}}"`}},
		{section: constants.CommandBackup, property: constants.ParameterTag, note: backupNote, examples: []string{"false", `"{{property}}"`}},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			propertyReplacer := func(s string) string {
				return strings.ReplaceAll(s, "{{property}}", test.property)
			}
			if test.examples == nil {
				test.examples = examples
			}
			if len(test.note) == 0 {
				test.note = note
			}

			info := customizeProperty(test.section, newResticPropertyInfo(test.property, restic.Option{}))

			assert.Equal(t, propertyReplacer(".\n"+test.note), info.Description())
			assert.Equal(t, collect.From(test.examples, propertyReplacer), info.ExampleValues())
			assert.Equal(t, test.format, info.Format())
			assert.True(t, info.CanBeString())
			assert.True(t, info.CanBeBool())
			assert.False(t, info.CanBeNumeric())
			assert.False(t, info.IsSingle())
		})
	}
}

func TestConfidentialProperty(t *testing.T) {
	var testType = struct {
		Simple ConfidentialValue            `mapstructure:"simple"`
		List   []ConfidentialValue          `mapstructure:"list"`
		Mapped map[string]ConfidentialValue `mapstructure:"mapped"`
	}{}

	set := propertySetFromType(reflect.TypeOf(testType))

	assert.ElementsMatch(t, []string{"simple", "list", "mapped"}, set.Properties())
	for _, name := range set.Properties() {
		t.Run(name+"/before", func(t *testing.T) {
			info := set.PropertyInfo(name)
			require.True(t, info.CanBePropertySet())
			if name == "mapped" {
				require.NotNil(t, info.PropertySet().OtherPropertyInfo())
				require.NotNil(t, info.PropertySet().OtherPropertyInfo().PropertySet())
				assert.Equal(t, "ConfidentialValue", info.PropertySet().OtherPropertyInfo().PropertySet().TypeName())
			} else {
				assert.Equal(t, "ConfidentialValue", info.PropertySet().TypeName())
			}
			assert.False(t, info.CanBeString())
			assert.False(t, info.IsMultiType())
		})
	}

	customizeProperties("any", set.properties)

	for _, name := range set.Properties() {
		t.Run(name, func(t *testing.T) {
			info := set.PropertyInfo(name)
			if name == "mapped" {
				require.True(t, info.CanBePropertySet())
				nested := info.PropertySet()
				assert.Nil(t, nested.OtherPropertyInfo())
				assert.Empty(t, nested.TypeName())
				assert.False(t, nested.IsClosed())
				assert.Empty(t, nested.Properties())
			} else {
				assert.True(t, info.CanBeString())
				assert.False(t, info.CanBePropertySet())
				assert.False(t, info.IsMultiType())
			}
		})
	}
}

func TestDeprecatedSection(t *testing.T) {
	var testType = struct {
		ScheduleBaseSection `mapstructure:",squash" deprecated:"true"`
	}{}

	set := propertySetFromType(reflect.TypeOf(testType))
	require.False(t, set.PropertyInfo("schedule").IsDeprecated())

	customizeProperties("any", set.properties)
	require.True(t, set.PropertyInfo("schedule").IsDeprecated())
}

func TestHelpIsExcluded(t *testing.T) {
	assert.True(t, isExcluded("*", "help"))
	assert.False(t, isExcluded("*", "any-other"))
	assert.False(t, isExcluded("named-section", "help"))
}
