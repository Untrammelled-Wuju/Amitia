package migration

import "testing"

func TestReleasedMigrationChecksums(t *testing.T) {
	expected := map[string]string{
		"001":          "bffc083ef1fe063c267db20517cda0f2084a9d5ee67e94f8ece2917339404e27",
		"002":          "d942ea1a3bb0f576e09b46a8c27feca2a9438f2f88c443ec6f070bf9088df6fc",
		"003":          "84d4ae791b018556390f61a7af448dcd7532fe0e41621a9b8469c8343cc81918",
		"004":          "b11e2fcb9bc79fa50f5fd2766edf8cd8cae3a61432dfb6dd069ea3c8661b4a57",
		"005":          "66afcc9e8af0ecc7e147977f30ee23d4436333f99b6d32c83731d6d88aa69d3b",
		"006":          "568b930c05ffba566b772a87b2682bf37ca4ed2356c166dc3ae5ae97d7365e2c",
		"017":          "b7eb16662292998f147104dbe18dfad658659aeaca580a84e2c7eb084a86ff81",
		"202607010001": "2ea95a62ad598ddef93601a628175168ae13660aa61c47c3223ebe5e45bc4eda",
		"202607010002": "91c6056b4754c0f9575eaf07a260ee5c2c6e6dfead82e6c41c083763d1987c62",
		"202607010003": "9a483a951428a1989212154199fcff461cee7849efedfed4049fdb080622f578",
		"202607010004": "8a2714d2aac7144f000f36b87507cd24f23179b9c6ab2d8a8b54e942fbe93ada",
		"202607010005": "4d608d1bfbedb38bef7b10d47d15fe82d8d7a26183dc59df5f9d1210dc52d0c8",
		"202607010069": "a398213ea33399de6f69360dddfd8f8c6f13978968308c8c7d20349be1b3f53a",
		"202607010102": "c285ffd9ff5bd50d36120756b3f4184c5f31e9e737a1281fbde97ebcd1628890",
		"202607030000": "147f91eaafa7413b3b7a8b94294d0208578ee3009133f6e13aba2e1bdd57d0db",
		"202607030001": "79a8a8f2d904cbdcd1b2cd09ce39269e1c2f16ffe20c22c8c59b17301a8c9414",
		"202607030002": "7266be8fdb160fb0781a259c00060e1c84cc50ae858618907805aa0fbbbe780b",
		"202607040001": "e5af76b522fd54202f6d0225705327bebef4e781a26796c7e1bc059dad92c936",
		"202607090001": "399b307e5d9dd6a7a7a9899e49cbd2b4e6eae2ac99e12719cd29e5066ed70524",
	}

	actual := make(map[string]string)
	runner := Runner{}
	for _, item := range DefaultMigrations() {
		checksum, err := runner.computeMigrationChecksum(item)
		if err != nil {
			t.Fatalf("compute checksum %s: %v", item.Version, err)
		}
		actual[item.Version] = checksum
	}

	for version, checksum := range expected {
		if actual[version] != checksum {
			t.Errorf("migration %s checksum = %s, want %s", version, actual[version], checksum)
		}
	}
}
