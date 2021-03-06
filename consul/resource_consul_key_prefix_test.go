package consul

import (
	"fmt"
	"regexp"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestAccConsulKeyPrefix_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckConsulKeyPrefixKeyAbsent("species"),
			testAccCheckConsulKeyPrefixKeyAbsent("meat"),
			testAccCheckConsulKeyPrefixKeyAbsent("cheese"),
			testAccCheckConsulKeyPrefixKeyAbsent("bread"),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccConsulKeyPrefixConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckConsulKeyPrefixKeyValue("cheese", "chevre", 0),
					testAccCheckConsulKeyPrefixKeyValue("bread", "baguette", 0),
					testAccCheckConsulKeyPrefixKeyValue("condiment/first", "tomato", 2),
					testAccCheckConsulKeyPrefixKeyValue("condiment/second", "salad", 4),
					testAccCheckConsulKeyPrefixKeyAbsent("species"),
					testAccCheckConsulKeyPrefixKeyAbsent("meat"),
				),
			},
			{
				Config:             testAccConsulKeyPrefixConfig,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					// This will add a rogue key that Terraform isn't
					// expecting, causing a non-empty plan that wants
					// to remove it.
					testAccAddConsulKeyPrefixRogue("species", "gorilla"),
				),
			},
			{
				Config: testAccConsulKeyPrefixConfig_Update,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckConsulKeyPrefixKeyValue("meat", "ham", 0),
					testAccCheckConsulKeyPrefixKeyValue("bread", "batard", 0),
					testAccCheckConsulKeyPrefixKeyAbsent("condiment/first"),
					testAccCheckConsulKeyPrefixKeyAbsent("condiment/second"),
					testAccCheckConsulKeyPrefixKeyAbsent("cheese"),
					testAccCheckConsulKeyPrefixKeyAbsent("species"),
				),
			},
			{
				Config:             testAccConsulKeyPrefixConfig_Update,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					testAccAddConsulKeyPrefixRogue("species", "gorilla"),
				),
			},
		},
	})
}

func TestAccCheckConsulKeyPrefix_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccConsulKeyPrefixConfig_Update,
			},
			{
				Config:            testAccConsulKeyPrefixConfig_Update,
				ResourceName:      "consul_key_prefix.app",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccConsulKeyPrefix_namespaceCE(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		PreCheck:  func() { skipTestOnConsulEnterpriseEdition(t) },
		Steps: []resource.TestStep{
			{
				Config:      testAccConsulKeyPrefixConfig_namespaceCE,
				ExpectError: regexp.MustCompile("Unexpected response code: 400"),
			},
		},
	})
}

func TestAccConsulKeyPrefix_namespaceEE(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		PreCheck:  func() { skipTestOnConsulCommunityEdition(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccConsulKeyPrefixConfig_namespaceEE,
			},
		},
	})
}

func testAccCheckConsulKeyPrefixDestroy(s *terraform.State) error {
	client := getClient(testAccProvider.Meta())
	kv := client.KV()
	opts := &consulapi.QueryOptions{Datacenter: "dc1"}
	pair, _, err := kv.Get("test/set", opts)
	if err != nil {
		return err
	}
	if pair != nil {
		return fmt.Errorf("Key still exists: %#v", pair)
	}
	return nil
}

func testAccCheckConsulKeyPrefixKeyAbsent(name string) resource.TestCheckFunc {
	fullName := "prefix_test/" + name
	return func(s *terraform.State) error {
		client := getClient(testAccProvider.Meta())
		kv := client.KV()
		opts := &consulapi.QueryOptions{Datacenter: "dc1"}
		pair, _, err := kv.Get(fullName, opts)
		if err != nil {
			return err
		}
		if pair != nil {
			return fmt.Errorf("key '%s' exists, but shouldn't", fullName)
		}
		return nil
	}
}

// This one is actually not a check, but rather a mutation step. It writes
// a value directly into Consul, bypassing our Terraform resource.
func testAccAddConsulKeyPrefixRogue(name, value string) resource.TestCheckFunc {
	fullName := "prefix_test/" + name
	return func(s *terraform.State) error {
		client := getClient(testAccProvider.Meta())
		kv := client.KV()
		opts := &consulapi.WriteOptions{Datacenter: "dc1"}
		pair := &consulapi.KVPair{
			Key:   fullName,
			Value: []byte(value),
		}
		_, err := kv.Put(pair, opts)
		return err
	}
}

func testAccCheckConsulKeyPrefixKeyValue(name, value string, flags uint64) resource.TestCheckFunc {
	fullName := "prefix_test/" + name
	return func(s *terraform.State) error {
		client := getClient(testAccProvider.Meta())
		kv := client.KV()
		opts := &consulapi.QueryOptions{Datacenter: "dc1"}
		pair, _, err := kv.Get(fullName, opts)
		if err != nil {
			return err
		}
		if pair == nil {
			return fmt.Errorf("key %v doesn't exist, but should", fullName)
		}
		if string(pair.Value) != value {
			return fmt.Errorf("key %v has value %v; want %v", fullName, pair.Value, value)
		}
		if pair.Flags != flags {
			return fmt.Errorf("key %v has flags %v; want %v", fullName, pair.Flags, flags)
		}
		return nil
	}
}

const testAccConsulKeyPrefixConfig = `
resource "consul_key_prefix" "app" {
	datacenter = "dc1"

    path_prefix = "prefix_test/"

    subkeys = {
        cheese = "chevre"
        bread = "baguette"
	}

	subkey {
		path  = "condiment/first"
		value = "tomato"
		flags = 2
	}

	subkey {
		path  = "condiment/second"
		value = "salad"
		flags = 4
	}
}
`

const testAccConsulKeyPrefixConfig_Update = `
resource "consul_key_prefix" "app" {
	datacenter = "dc1"

    path_prefix = "prefix_test/"

    subkeys = {
        bread = "batard"
        meat = "ham"
    }
}
`

const testAccConsulKeyPrefixConfig_namespaceCE = `
resource "consul_key_prefix" "test" {
  path_prefix = "prefix_test/"
  namespace   = "test-key-prefix"

  subkeys = {
    bread = "batard"
    meat = "ham"
  }
}`

const testAccConsulKeyPrefixConfig_namespaceEE = `
resource "consul_namespace" "test" {
  name = "test-key-prefix"
}

resource "consul_key_prefix" "test" {
  path_prefix = "prefix_test/"
  namespace   = consul_namespace.test.name

  subkeys = {
    bread = "batard"
    meat = "ham"
  }
}`
