/*
   Copyright 2022 Erigon contributors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package commitment

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/length"
)

func Test_HexPatriciaHashed_ResetThenSingularUpdates(t *testing.T) {
	ms := NewMockState(t)
	hph := NewHexPatriciaHashed(1, ms.branchFn, ms.accountFn, ms.storageFn)
	hph.SetTrace(false)
	plainKeys, updates := NewUpdateBuilder().
		Balance("00", 4).
		Balance("01", 5).
		Balance("02", 6).
		Balance("03", 7).
		Balance("04", 8).
		Storage("04", "01", "0401").
		Storage("03", "56", "050505").
		Storage("03", "57", "060606").
		Balance("05", 9).
		Storage("05", "02", "8989").
		Storage("05", "04", "9898").
		Build()

	err := ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	firstRootHash, branchNodeUpdates, err := hph.ProcessUpdates(plainKeys, updates)
	require.NoError(t, err)

	t.Logf("root hash %x\n", firstRootHash)

	ms.applyBranchNodeUpdates(branchNodeUpdates)

	fmt.Printf("1. Generated updates\n")
	//renderUpdates(branchNodeUpdates)

	// More updates
	hph.Reset()
	//hph.SetTrace(true)
	plainKeys, updates = NewUpdateBuilder().
		Storage("03", "58", "050506").
		Build()
	err = ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	secondRootHash, branchNodeUpdates, err := hph.ProcessKeys(plainKeys)
	require.NoError(t, err)
	require.NotEqualValues(t, firstRootHash, secondRootHash)
	t.Logf("second root hash %x\n", secondRootHash)

	ms.applyBranchNodeUpdates(branchNodeUpdates)
	fmt.Printf("2. Generated single update\n")
	renderUpdates(branchNodeUpdates)

	// More updates
	hph.Reset()
	//hph.SetTrace(true)
	plainKeys, updates = NewUpdateBuilder().
		Storage("03", "58", "020807").
		Build()
	fmt.Printf("3. Generated single update %s\n", updates[0].String())
	err = ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	thirdRootHash, branchNodeUpdates, err := hph.ProcessKeys(plainKeys)
	t.Logf("third root hash %x\n", secondRootHash)
	require.NoError(t, err)
	require.NotEqualValues(t, secondRootHash, thirdRootHash)
	renderUpdates(branchNodeUpdates)

	ms.applyBranchNodeUpdates(branchNodeUpdates)
	fmt.Printf("3. Generated single update\n")
	//renderUpdates(branchNodeUpdates)
}

func Test_HexPatriciaHashed_EmptyUpdate(t *testing.T) {
	ms := NewMockState(t)
	hph := NewHexPatriciaHashed(1, ms.branchFn, ms.accountFn, ms.storageFn)
	hph.SetTrace(false)
	plainKeys, updates := NewUpdateBuilder().
		Balance("00", 4).
		Nonce("00", 246462653).
		Balance("01", 5).
		CodeHash("03", "aaaaaaaaaaf7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a870").
		Delete("00").
		Storage("04", "01", "0401").
		Storage("03", "56", "050505").
		Build()

	err := ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	hashBeforeEmptyUpdate, branchNodeUpdates, err := hph.ProcessKeys(plainKeys)
	require.NoError(t, err)
	require.NotEmpty(t, hashBeforeEmptyUpdate)

	ms.applyBranchNodeUpdates(branchNodeUpdates)

	fmt.Println("1. Updates applied")
	//renderUpdates(branchNodeUpdates)

	// generate empty updates and do NOT reset tree
	//hph.SetTrace(true)

	plainKeys, updates = NewUpdateBuilder().Build()

	err = ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	hashAfterEmptyUpdate, branchNodeUpdates, err := hph.ProcessKeys(plainKeys)
	require.NoError(t, err)

	ms.applyBranchNodeUpdates(branchNodeUpdates)
	fmt.Println("2. Empty updates applied without state reset")

	require.EqualValues(t, hashBeforeEmptyUpdate, hashAfterEmptyUpdate)
}

func Test_HexPatriciaHashed_UniqueRepresentation2(t *testing.T) {
	ms := NewMockState(t)
	ms2 := NewMockState(t)

	plainKeys, updates := NewUpdateBuilder().
		Balance("71562b71999873db5b286df957af199ec94617f7", 999860099).
		Nonce("71562b71999873db5b286df957af199ec94617f7", 3).
		Balance("3a220f351252089d385b29beca14e27f204c296a", 900234).
		Balance("0000000000000000000000000000000000000000", 2000000000000138901).
		//Balance("0000000000000000000000000000000000000000", 4000000000000138901).
		Build()

	trieOne := NewHexPatriciaHashed(20, ms.branchFn, ms.accountFn, ms.storageFn)
	trieTwo := NewHexPatriciaHashed(20, ms2.branchFn, ms2.accountFn, ms2.storageFn)

	//trieOne.SetTrace(true)
	//trieTwo.SetTrace(true)

	// single sequential update
	roots := make([][]byte, 0)
	fmt.Printf("1. Trie sequential update generated following branch updates\n")

	var ra, rb []byte
	{
		if err := ms.applyPlainUpdates(plainKeys, updates); err != nil {
			t.Fatal(err)
		}

		rh, branchNodeUpdates, err := trieOne.ProcessKeys(plainKeys)
		require.NoError(t, err)
		ms.applyBranchNodeUpdates(branchNodeUpdates)
		//renderUpdates(branchNodeUpdates)

		ra = common.Copy(rh)
	}
	{
		err := ms2.applyPlainUpdates(plainKeys, updates)
		require.NoError(t, err)

		fmt.Printf("\n2. Trie batch update generated following branch updates\n")
		// batch update
		rh, branchNodeUpdatesTwo, err := trieTwo.ProcessKeys(plainKeys)
		require.NoError(t, err)
		ms2.applyBranchNodeUpdates(branchNodeUpdatesTwo)
		//renderUpdates(branchNodeUpdatesTwo)

		rb = common.Copy(rh)
	}
	require.EqualValues(t, ra, rb)

	plainKeys, updates = NewUpdateBuilder().
		//Balance("71562b71999873db5b286df957af199ec94617f7", 999860099).
		//Nonce("71562b71999873db5b286df957af199ec94617f7", 3).
		//Balance("3a220f351252089d385b29beca14e27f204c296a", 900234).
		//Balance("0000000000000000000000000000000000000000", 2000000000000138901).
		Balance("0000000000000000000000000000000000000000", 4000000000000138901).
		Build()

	if err := ms.applyPlainUpdates(plainKeys, updates); err != nil {
		t.Fatal(err)
	}

	sequentialRoot, branchNodeUpdates, err := trieOne.ProcessKeys(plainKeys)
	require.NoError(t, err)
	roots = append(roots, sequentialRoot)
	ms.applyBranchNodeUpdates(branchNodeUpdates)
	//renderUpdates(branchNodeUpdates)

	plainKeys, updates = NewUpdateBuilder().
		Balance("71562b71999873db5b286df957af199ec94617f7", 999860099).
		Nonce("71562b71999873db5b286df957af199ec94617f7", 3).
		Balance("3a220f351252089d385b29beca14e27f204c296a", 900234).
		//Balance("0000000000000000000000000000000000000000", 2000000000000138901).
		Balance("0000000000000000000000000000000000000000", 4000000000000138901).
		Build()

	err = ms2.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	fmt.Printf("\n2. Trie batch update generated following branch updates\n")
	// batch update
	batchRoot, branchNodeUpdatesTwo, err := trieTwo.ProcessKeys(plainKeys)
	require.NoError(t, err)
	//renderUpdates(branchNodeUpdatesTwo)

	fmt.Printf("\n sequential roots:\n")
	for i, rh := range roots {
		fmt.Printf("%2d %+v\n", i, hex.EncodeToString(rh))
	}

	ms2.applyBranchNodeUpdates(branchNodeUpdatesTwo)

	require.EqualValues(t, batchRoot, roots[len(roots)-1],
		"expected equal roots, got sequential [%v] != batch [%v]", hex.EncodeToString(roots[len(roots)-1]), hex.EncodeToString(batchRoot))
	require.Lenf(t, batchRoot, 32, "root hash length should be equal to 32 bytes")
}

// Ordering is crucial for trie. since trie do hashing by itself and reorder updates inside Process{Keys,Updates}, have to reorder them for some tests
func sortUpdatesByHashIncrease(t *testing.T, hph *HexPatriciaHashed, plainKeys [][]byte, updates []Update) ([][]byte, []Update) {
	t.Helper()

	for i, pk := range plainKeys {
		updates[i].hashedKey = hph.hashAndNibblizeKey(pk)
		updates[i].plainKey = pk
	}

	sort.Slice(updates, func(i, j int) bool {
		return bytes.Compare(updates[i].hashedKey, updates[j].hashedKey) < 0
	})

	pks := make([][]byte, len(updates))
	for i, u := range updates {
		pks[i] = u.plainKey
	}
	return pks, updates
}

// TODO(awskii)
func Test_HexPatriciaHashed_BrokenUniqueRepr(t *testing.T) {
	//t.Skip("awskii should fix issue with insertion of storage before account")

	uniqTest := func(t *testing.T, sortHashedKeys bool, trace bool) {
		t.Helper()

		stateSeq := NewMockState(t)
		stateBatch := NewMockState(t)

		plainKeys, updates := NewUpdateBuilder().
			Balance("03", 7).
			Storage("03", "87", "060606").
			//Balance("68ee6c0e9cdc73b2b2d52dbd79f19d24fe25e2f9", 4).
			//Storage("8e5476fc5990638a4fb0b5fd3f61bb4b5c5f395e", "24f3a02dc65eda502dbf75919e795458413d3c45b38bb35b51235432707900ed", "0401").
			Build()

		trieSequential := NewHexPatriciaHashed(1, stateSeq.branchFn, stateSeq.accountFn, stateSeq.storageFn)
		trieBatch := NewHexPatriciaHashed(1, stateBatch.branchFn, stateBatch.accountFn, stateBatch.storageFn)

		if sortHashedKeys {
			plainKeys, updates = sortUpdatesByHashIncrease(t, trieSequential, plainKeys, updates)
		}

		trieSequential.SetTrace(trace)
		trieBatch.SetTrace(trace)

		roots := make([][]byte, 0)
		// branchNodeUpdatesOne := make(map[string]BranchData)
		fmt.Printf("1. Trie sequential update generated following branch updates\n")
		for i := 0; i < len(updates); i++ { // apply updates one by one
			if err := stateSeq.applyPlainUpdates(plainKeys[i:i+1], updates[i:i+1]); err != nil {
				t.Fatal(err)
			}

			sequentialRoot, branchNodeUpdates, err := trieSequential.ProcessKeys(plainKeys[i : i+1])
			require.NoError(t, err)
			roots = append(roots, sequentialRoot)

			stateSeq.applyBranchNodeUpdates(branchNodeUpdates)
			if trieSequential.trace {
				renderUpdates(branchNodeUpdates)
			}
		}

		fmt.Printf("\n sequential roots:\n")
		for i, rh := range roots {
			fmt.Printf("%2d %+v\n", i, hex.EncodeToString(rh))
		}

		err := stateBatch.applyPlainUpdates(plainKeys, updates)
		require.NoError(t, err)

		fmt.Printf("\n2. Trie batch update generated following branch updates\n")
		// batch update
		batchRoot, branchNodeUpdatesTwo, err := trieBatch.ProcessKeys(plainKeys)
		require.NoError(t, err)
		if trieBatch.trace {
			renderUpdates(branchNodeUpdatesTwo)
		}
		stateBatch.applyBranchNodeUpdates(branchNodeUpdatesTwo)
		fmt.Printf("batch root is %x\n", batchRoot)

		require.EqualValues(t, batchRoot, roots[len(roots)-1],
			"expected equal roots, got sequential [%v] != batch [%v]", hex.EncodeToString(roots[len(roots)-1]), hex.EncodeToString(batchRoot))
		require.Lenf(t, batchRoot, 32, "root hash length should be equal to 32 bytes")

	}

	// Same PLAIN prefix is not necessary while HASHED CPL>0 is required
	t.Run("InsertStorageWhenCPL==0", func(t *testing.T) {
		// processed 03.87 then 03
		uniqTest(t, true, true)
	})
	t.Run("InsertStorageWhenCPL>0", func(t *testing.T) {
		// processed 03 then 03.87
		uniqTest(t, false, true)
	})
}

func Test_HexPatriciaHashed_UniqueRepresentation(t *testing.T) {
	//t.Skip("has to fix Test_HexPatriciaHashed_BrokenUniqueRepr first to get this green")

	stateSeq := NewMockState(t)
	stateBatch := NewMockState(t)

	plainKeys, updates := NewUpdateBuilder().
		Balance("68ee6c0e9cdc73b2b2d52dbd79f19d24fe25e2f9", 4).
		Balance("18f4dcf2d94402019d5b00f71d5f9d02e4f70e40", 900234).
		Balance("8e5476fc5990638a4fb0b5fd3f61bb4b5c5f395e", 1233).
		Storage("8e5476fc5990638a4fb0b5fd3f61bb4b5c5f395e", "24f3a02dc65eda502dbf75919e795458413d3c45b38bb35b51235432707900ed", "0401").
		Balance("27456647f49ba65e220e86cba9abfc4fc1587b81", 065606).
		Balance("b13363d527cdc18173c54ac5d4a54af05dbec22e", 4*1e17).
		Balance("d995768ab23a0a333eb9584df006da740e66f0aa", 5).
		Balance("eabf041afbb6c6059fbd25eab0d3202db84e842d", 6).
		Balance("93fe03620e4d70ea39ab6e8c0e04dd0d83e041f2", 7).
		Storage("ba7a3b7b095d3370c022ca655c790f0c0ead66f5", "0fa41642c48ecf8f2059c275353ce4fee173b3a8ce5480f040c4d2901603d14e", "050505").
		Balance("a8f8d73af90eee32dc9729ce8d5bb762f30d21a4", 9*1e16).
		//Storage("93fe03620e4d70ea39ab6e8c0e04dd0d83e041f2", "de3fea338c95ca16954e80eb603cd81a261ed6e2b10a03d0c86cf953fe8769a4", "060606").
		Balance("14c4d3bba7f5009599257d3701785d34c7f2aa27", 6*1e18).
		Nonce("18f4dcf2d94402019d5b00f71d5f9d02e4f70e40", 169356).
		//Storage("a8f8d73af90eee32dc9729ce8d5bb762f30d21a4", "9f49fdd48601f00df18ebc29b1264e27d09cf7cbd514fe8af173e534db038033", "8989").
		//Storage("68ee6c0e9cdc73b2b2d52dbd79f19d24fe25e2f9", "d1664244ae1a8a05f8f1d41e45548fbb7aa54609b985d6439ee5fd9bb0da619f", "9898").
		Build()

	trieSequential := NewHexPatriciaHashed(length.Addr, stateSeq.branchFn, stateSeq.accountFn, stateSeq.storageFn)
	trieBatch := NewHexPatriciaHashed(length.Addr, stateBatch.branchFn, stateBatch.accountFn, stateBatch.storageFn)

	plainKeys, updates = sortUpdatesByHashIncrease(t, trieSequential, plainKeys, updates)

	trieSequential.SetTrace(true)
	trieBatch.SetTrace(true)

	roots := make([][]byte, 0)
	fmt.Printf("1. Trie sequential update generated following branch updates\n")
	for i := 0; i < len(updates); i++ { // apply updates one by one
		if err := stateSeq.applyPlainUpdates(plainKeys[i:i+1], updates[i:i+1]); err != nil {
			t.Fatal(err)
		}

		sequentialRoot, branchNodeUpdates, err := trieSequential.ProcessKeys(plainKeys[i : i+1])
		require.NoError(t, err)
		roots = append(roots, sequentialRoot)

		stateSeq.applyBranchNodeUpdates(branchNodeUpdates)
		if trieSequential.trace {
			renderUpdates(branchNodeUpdates)
		}
	}

	fmt.Printf("\n sequential roots:\n")
	for i, rh := range roots {
		fmt.Printf("%2d %+v\n", i, hex.EncodeToString(rh))
	}

	err := stateBatch.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	fmt.Printf("\n2. Trie batch update generated following branch updates\n")
	// batch update
	batchRoot, branchNodeUpdatesTwo, err := trieBatch.ProcessKeys(plainKeys)
	require.NoError(t, err)
	if trieBatch.trace {
		renderUpdates(branchNodeUpdatesTwo)
	}
	stateBatch.applyBranchNodeUpdates(branchNodeUpdatesTwo)
	fmt.Printf("batch root is %x\n", batchRoot)

	require.EqualValues(t, batchRoot, roots[len(roots)-1],
		"expected equal roots, got sequential [%v] != batch [%v]", hex.EncodeToString(roots[len(roots)-1]), hex.EncodeToString(batchRoot))
	require.Lenf(t, batchRoot, 32, "root hash length should be equal to 32 bytes")
}

func Test_HexPatriciaHashed_Sepolia(t *testing.T) {
	ms := NewMockState(t)

	type TestData struct {
		balances     map[string][]byte
		expectedRoot string
	}

	tests := []TestData{
		{
			expectedRoot: "5eb6e371a698b8d68f665192350ffcecbbbf322916f4b51bd79bb6887da3f494",
			balances: map[string][]byte{
				"a2a6d93439144ffe4d27c9e088dcd8b783946263": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"bc11295936aa79d594139de1b2e12629414f3bdb": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"7cf5b79bfe291a67ab02b393e456ccc4c266f753": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"aaec86394441f915bce3e6ab399977e9906f3b69": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"f47cae1cf79ca6758bfc787dbd21e6bdbe7112b8": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"d7eddb78ed295b3c9629240e8924fb8d8874ddd8": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"8b7f0977bb4f0fbe7076fa22bc24aca043583f5e": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"e2e2659028143784d557bcec6ff3a0721048880a": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"d9a5179f091d85051d3c982785efd1455cec8699": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"beef32ca5b9a198d27b4e02f4c70439fe60356cf": {0xd3, 0xc2, 0x1b, 0xce, 0xcc, 0xed, 0xa1, 0x00, 0x00, 0x00},
				"0000006916a87b82333f4245046623b23794c65c": {0x08, 0x45, 0x95, 0x16, 0x14, 0x01, 0x48, 0x4a, 0x00, 0x00, 0x00},
				"b21c33de1fab3fa15499c62b59fe0cc3250020d1": {0x52, 0xb7, 0xd2, 0xdc, 0xc8, 0x0c, 0xd2, 0xe4, 0x00, 0x00, 0x00},
				"10f5d45854e038071485ac9e402308cf80d2d2fe": {0x52, 0xb7, 0xd2, 0xdc, 0xc8, 0x0c, 0xd2, 0xe4, 0x00, 0x00, 0x00},
				"d7d76c58b3a519e9fa6cc4d22dc017259bc49f1e": {0x52, 0xb7, 0xd2, 0xdc, 0xc8, 0x0c, 0xd2, 0xe4, 0x00, 0x00, 0x00},
				"799d329e5f583419167cd722962485926e338f4a": {0x0d, 0xe0, 0xb6, 0xb3, 0xa7, 0x64, 0x00, 0x00},
			},
		},
		{
			expectedRoot: "c91d4ecd59dce3067d340b3aadfc0542974b4fb4db98af39f980a91ea00db9dc",
			balances: map[string][]byte{
				"2f14582947e292a2ecd20c430b46f2d27cfe213c": {0x1B, 0xC1, 0x6D, 0x67, 0x4E, 0xC8, 0x00, 0x00},
			},
		},
		{
			expectedRoot: "c91d4ecd59dce3067d340b3aadfc0542974b4fb4db98af39f980a91ea00db9dc",
			balances:     map[string][]byte{},
		},
	}

	hph := NewHexPatriciaHashed(length.Addr, ms.branchFn, ms.accountFn, ms.storageFn)
	//hph.SetTrace(true)

	for _, testData := range tests {
		builder := NewUpdateBuilder()

		for address, balance := range testData.balances {
			builder.IncrementBalance(address, balance)
		}
		plainKeys, updates := builder.Build()

		if err := ms.applyPlainUpdates(plainKeys, updates); err != nil {
			t.Fatal(err)
		}

		rootHash, branchNodeUpdates, err := hph.ProcessKeys(plainKeys)
		if err != nil {
			t.Fatal(err)
		}
		ms.applyBranchNodeUpdates(branchNodeUpdates)

		require.EqualValues(t, testData.expectedRoot, fmt.Sprintf("%x", rootHash))
	}
}

func Test_Cell_EncodeDecode(t *testing.T) {
	rnd := rand.New(rand.NewSource(time.Now().UnixMilli()))
	first := &Cell{
		Nonce:         rnd.Uint64(),
		hl:            length.Hash,
		StorageLen:    rnd.Intn(33),
		apl:           length.Addr,
		spl:           length.Addr + length.Hash,
		downHashedLen: rnd.Intn(129),
		extLen:        rnd.Intn(65),
		downHashedKey: [128]byte{},
		extension:     [64]byte{},
		spk:           [52]byte{},
		h:             [32]byte{},
		CodeHash:      [32]byte{},
		Storage:       [32]byte{},
		apk:           [20]byte{},
	}
	b := uint256.NewInt(rnd.Uint64())
	first.Balance = *b

	rnd.Read(first.downHashedKey[:first.downHashedLen])
	rnd.Read(first.extension[:first.extLen])
	rnd.Read(first.spk[:])
	rnd.Read(first.apk[:])
	rnd.Read(first.h[:])
	rnd.Read(first.CodeHash[:])
	rnd.Read(first.Storage[:first.StorageLen])
	if rnd.Intn(100) > 50 {
		first.Delete = true
	}

	second := &Cell{}
	second.Decode(first.Encode())

	require.EqualValues(t, first.downHashedLen, second.downHashedLen)
	require.EqualValues(t, first.downHashedKey[:], second.downHashedKey[:])
	require.EqualValues(t, first.apl, second.apl)
	require.EqualValues(t, first.spl, second.spl)
	require.EqualValues(t, first.hl, second.hl)
	require.EqualValues(t, first.apk[:], second.apk[:])
	require.EqualValues(t, first.spk[:], second.spk[:])
	require.EqualValues(t, first.h[:], second.h[:])
	require.EqualValues(t, first.extension[:first.extLen], second.extension[:second.extLen])
	// encode doesnt code Nonce, Balance, CodeHash and Storage
	require.EqualValues(t, first.Delete, second.Delete)
}

func Test_HexPatriciaHashed_StateEncode(t *testing.T) {
	//trie := NewHexPatriciaHashed(length.Hash, nil, nil, nil)
	var s state
	s.Root = make([]byte, 128)
	rnd := rand.New(rand.NewSource(42))

	n, err := rnd.Read(s.Root[:])
	require.NoError(t, err)
	require.EqualValues(t, len(s.Root), n)
	s.RootPresent = true
	s.RootTouched = true
	s.RootChecked = true

	for i := 0; i < len(s.Depths); i++ {
		s.Depths[i] = rnd.Intn(256)
	}
	for i := 0; i < len(s.TouchMap); i++ {
		s.TouchMap[i] = uint16(rnd.Intn(1<<16 - 1))
	}
	for i := 0; i < len(s.AfterMap); i++ {
		s.AfterMap[i] = uint16(rnd.Intn(1<<16 - 1))
	}
	for i := 0; i < len(s.BranchBefore); i++ {
		if rnd.Intn(100) > 49 {
			s.BranchBefore[i] = true
		}
	}

	enc, err := s.Encode(nil)
	require.NoError(t, err)
	require.NotEmpty(t, enc)

	var s1 state
	err = s1.Decode(enc)
	require.NoError(t, err)

	require.EqualValues(t, s.Root[:], s1.Root[:])
	require.EqualValues(t, s.Depths[:], s1.Depths[:])
	require.EqualValues(t, s.AfterMap[:], s1.AfterMap[:])
	require.EqualValues(t, s.TouchMap[:], s1.TouchMap[:])
	require.EqualValues(t, s.BranchBefore[:], s1.BranchBefore[:])
	require.EqualValues(t, s.RootTouched, s1.RootTouched)
	require.EqualValues(t, s.RootPresent, s1.RootPresent)
	require.EqualValues(t, s.RootChecked, s1.RootChecked)
}

func Test_HexPatriciaHashed_StateEncodeDecodeSetup(t *testing.T) {
	ms := NewMockState(t)

	plainKeys, updates := NewUpdateBuilder().
		Balance("f5", 4).
		Balance("ff", 900234).
		Balance("03", 7).
		Storage("03", "56", "050505").
		Balance("05", 9).
		Storage("03", "87", "060606").
		Balance("b9", 6).
		Nonce("ff", 169356).
		Storage("05", "02", "8989").
		Storage("f5", "04", "9898").
		Build()

	before := NewHexPatriciaHashed(1, ms.branchFn, ms.accountFn, ms.storageFn)
	after := NewHexPatriciaHashed(1, ms.branchFn, ms.accountFn, ms.storageFn)

	err := ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	rhBefore, branchUpdates, err := before.ProcessKeys(plainKeys)
	require.NoError(t, err)
	ms.applyBranchNodeUpdates(branchUpdates)

	state, err := before.EncodeCurrentState(nil)
	require.NoError(t, err)

	err = after.SetState(state)
	require.NoError(t, err)

	rhAfter, err := after.RootHash()
	require.NoError(t, err)
	require.EqualValues(t, rhBefore, rhAfter)

	// create new update and apply it to both tries
	nextPK, nextUpdates := NewUpdateBuilder().
		Nonce("ff", 4).
		Balance("b9", 6000000000).
		Balance("ad", 8000000000).
		Build()

	err = ms.applyPlainUpdates(nextPK, nextUpdates)
	require.NoError(t, err)

	rh2Before, branchUpdates, err := before.ProcessKeys(nextPK)
	require.NoError(t, err)
	ms.applyBranchNodeUpdates(branchUpdates)

	rh2After, branchUpdates, err := after.ProcessKeys(nextPK)
	require.NoError(t, err)

	_ = branchUpdates

	require.EqualValues(t, rh2Before, rh2After)
}

func Test_HexPatriciaHashed_StateRestoreAndContinue(t *testing.T) {
	ms := NewMockState(t)

	plainKeys, updates := NewUpdateBuilder().
		Balance("f5", 4).
		Balance("ff", 900234).
		Build()

	trieOne := NewHexPatriciaHashed(1, ms.branchFn, ms.accountFn, ms.storageFn)
	err := ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	beforeRestore, branchNodeUpdatesOne, err := trieOne.ProcessKeys(plainKeys)
	require.NoError(t, err)

	//renderUpdates(branchNodeUpdatesOne)
	ms.applyBranchNodeUpdates(branchNodeUpdatesOne)

	buf, err := trieOne.EncodeCurrentState(nil)
	require.NoError(t, err)
	require.NotEmpty(t, buf)

	trieTwo := NewHexPatriciaHashed(1, ms.branchFn, ms.accountFn, ms.storageFn)
	err = trieTwo.SetState(buf)
	require.NoError(t, err)

	hashAfterRestore, err := trieTwo.RootHash()
	require.NoError(t, err)
	require.EqualValues(t, beforeRestore, hashAfterRestore)

	plainKeys, updates = NewUpdateBuilder().
		Balance("ff", 900234).
		Balance("04", 1233).
		Storage("04", "01", "0401").
		Balance("ba", 065606).
		Balance("00", 4).
		Balance("01", 5).
		Balance("02", 6).
		Balance("03", 7).
		Storage("03", "56", "050505").
		Balance("05", 9).
		Storage("03", "87", "060606").
		Balance("b9", 6).
		Nonce("ff", 169356).
		Storage("05", "02", "8989").
		Storage("f5", "04", "9898").
		Build()

	err = ms.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	beforeRestore, branchNodeUpdatesOne, err = trieOne.ProcessKeys(plainKeys)
	require.NoError(t, err)

	renderUpdates(branchNodeUpdatesOne)

	twoAfterRestore, branchNodeUpdatesTwo, err := trieTwo.ProcessKeys(plainKeys)
	require.NoError(t, err)

	_ = branchNodeUpdatesTwo

	ms.applyBranchNodeUpdates(branchNodeUpdatesOne)
	require.EqualValues(t, beforeRestore, twoAfterRestore)
}

func Test_HexPatriciaHashed_RestoreAndContinue(t *testing.T) {
	ms := NewMockState(t)
	ms2 := NewMockState(t)

	plainKeys, updates := NewUpdateBuilder().
		Balance("f5", 4).
		Balance("ff", 900234).
		Balance("04", 1233).
		Storage("04", "01", "0401").
		Balance("ba", 065606).
		Balance("00", 4).
		Balance("01", 5).
		Balance("02", 6).
		Balance("03", 7).
		Storage("03", "56", "050505").
		Balance("05", 9).
		Storage("03", "87", "060606").
		Balance("b9", 6).
		Nonce("ff", 169356).
		Storage("05", "02", "8989").
		Storage("f5", "04", "9898").
		Build()

	trieOne := NewHexPatriciaHashed(1, ms.branchFn, ms.accountFn, ms.storageFn)
	trieTwo := NewHexPatriciaHashed(1, ms2.branchFn, ms2.accountFn, ms2.storageFn)

	err := ms2.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	_ = updates

	beforeRestore, branchNodeUpdatesTwo, err := trieTwo.ProcessKeys(plainKeys)
	require.NoError(t, err)
	//renderUpdates(branchNodeUpdatesTwo)
	ms2.applyBranchNodeUpdates(branchNodeUpdatesTwo)

	buf, err := trieTwo.EncodeCurrentState(nil)
	require.NoError(t, err)
	require.NotEmpty(t, buf)

	err = trieOne.SetState(buf)
	require.NoError(t, err)
	fmt.Printf("rh %x\n", trieOne.root.h[:])
	require.EqualValues(t, beforeRestore[:], trieOne.root.h[:])

	hashAfterRestore, err := trieOne.RootHash()
	require.NoError(t, err)
	require.EqualValues(t, beforeRestore, hashAfterRestore)
}

func Test_HexPatriciaHashed_ProcessUpdates_UniqueRepresentation_AfterStateRestore(t *testing.T) {
	t.Skip("has to fix Test_HexPatriciaHashed_BrokenUniqueRepr first to get this green")

	seqState := NewMockState(t)
	batchState := NewMockState(t)

	plainKeys, updates := NewUpdateBuilder().
		Balance("f5", 4).
		Balance("ff", 900234).
		Balance("04", 1233).
		Storage("04", "01", "0401").
		Balance("ba", 065606).
		Balance("00", 4).
		Balance("01", 5).
		Balance("02", 6).
		Balance("03", 7).
		Storage("03", "56", "050505").
		Balance("05", 9).
		Storage("03", "87", "060606").
		Balance("b9", 6).
		Nonce("ff", 169356).
		Storage("05", "02", "8989").
		Storage("f5", "04", "9898").
		Build()

	sequential := NewHexPatriciaHashed(1, seqState.branchFn, seqState.accountFn, seqState.storageFn)
	batch := NewHexPatriciaHashed(1, batchState.branchFn, batchState.accountFn, batchState.storageFn)

	plainKeys, updates = sortUpdatesByHashIncrease(t, sequential, plainKeys, updates)

	batch.Reset()
	sequential.Reset()
	//sequential.SetTrace(true)
	//batch.SetTrace(true)

	// single sequential update
	roots := make([][]byte, 0)
	prevState := make([]byte, 0)
	fmt.Printf("1. Trie sequential update generated following branch updates\n")
	for i := 0; i < len(updates); i++ {
		if err := seqState.applyPlainUpdates(plainKeys[i:i+1], updates[i:i+1]); err != nil {
			t.Fatal(err)
		}
		if i == (len(updates) / 2) {
			sequential.Reset()
			sequential.ResetFns(seqState.branchFn, seqState.accountFn, seqState.storageFn)
			err := sequential.SetState(prevState)
			require.NoError(t, err)
		}

		sequentialRoot, branchNodeUpdates, err := sequential.ProcessKeys(plainKeys[i : i+1])
		require.NoError(t, err)
		roots = append(roots, sequentialRoot)

		if sequential.trace {
			renderUpdates(branchNodeUpdates)
		}
		seqState.applyBranchNodeUpdates(branchNodeUpdates)

		if i == (len(updates)/2 - 1) {
			prevState, err = sequential.EncodeCurrentState(nil)
			require.NoError(t, err)
		}
	}
	for i, sr := range roots {
		fmt.Printf("%d %x\n", i, sr)
	}

	err := batchState.applyPlainUpdates(plainKeys, updates)
	require.NoError(t, err)

	fmt.Printf("\n2. Trie batch update generated following branch updates\n")
	// batch update
	batchRoot, branchNodeUpdatesTwo, err := batch.ProcessKeys(plainKeys)
	require.NoError(t, err)
	if batch.trace {
		renderUpdates(branchNodeUpdatesTwo)
	}
	batchState.applyBranchNodeUpdates(branchNodeUpdatesTwo)

	require.EqualValues(t, batchRoot, roots[len(roots)-1],
		"expected equal roots, got sequential [%v] != batch [%v]", hex.EncodeToString(roots[len(roots)-1]), hex.EncodeToString(batchRoot))
	require.Lenf(t, batchRoot, 32, "root hash length should be equal to 32 bytes")
}
