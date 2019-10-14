package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-merkledag"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder/multisectorbuilder"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/types"
	go_sectorbuilder "github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/go-sectorbuilder/sealing_state"
)

var simplePieceNum int

func init() {
	rootCmd.AddCommand(SimpleSectorBuilderCmd)

	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderAddPieceCmd)
	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderGenPieceCmd)
	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderLsPiecesCmd)
	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderGetPiecesCmd)
	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderSealSectorsCmd)
	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderLsSectorsCmd)
	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderVerifySectorsPorepCmd)
	SimpleSectorBuilderCmd.AddCommand(SimpleSectorBuilderVerifySectorsPostCmd)

	SimpleSectorBuilderGenPieceCmd.Flags().IntVarP(&simplePieceNum, "piece-num", "n", 1, "The number of pieces to generate")
}

var SimpleSectorBuilderCmd = &cobra.Command{
	Use:   "simple-sector-builder",
	Short: "Commands for filecoin simple sector builder",
}

var SimpleSectorBuilderLsPiecesCmd = &cobra.Command{
	Use:   "ls-pieces",
	Short: "List all pieces",
	Run: func(cmd *cobra.Command, args []string) {
		SectorBuilderLsPiecesCmd.Run(cmd, args)
	},
}

var SimpleSectorBuilderGetPiecesCmd = &cobra.Command{
	Use:   "get-piece <cid> <file>",
	Short: "Get piece and save into file",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		SectorBuilderGetPiecesCmd.Run(cmd, args)
		var err error
		defer func() {
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}()
	},
}

var SimpleSectorBuilderAddPieceCmd = &cobra.Command{
	Use:   "add-piece <file>",
	Short: "Add piece",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := args[0]
		file, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		dag := openSectorBuilderPiecesDAG()
		defer dag.Close()
		ds := openMetaDatastore()

		nd, err := dag.dag.ImportData(context.Background(), file)
		if err != nil {
			panic(err)
		}
		err = ds.Put(makeKey(metaSectorBuilderPiecePrefix, nd.Cid().String()), nil)
		if err != nil {
			return
		}

		ds.Close()

		sb := openSimpleSectorBuilder()
		defer sb.Close()

		r, err := dag.dag.Cat(context.Background(), nd.Cid())
		if err != nil {
			return
		}
		t := time.Now()
		sectorID, err := sb.AddPiece(context.Background(), minerAddr, nd.Cid(), r.Size(), r)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Added piece %s into staging sector %d, took %v\n", nd.Cid(), sectorID, time.Since(t))
	},
}

var SimpleSectorBuilderGenPieceCmd = &cobra.Command{
	Use:   "generate-piece <file>",
	Short: "Generate piece",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSimpleSectorBuilder()
		defer sb.Close()

		for i := 0; i < simplePieceNum; i++ {
			pieceData := make([]byte, sb.MaxBytesPerSector.Uint64())
			_, err := io.ReadFull(rand.Reader, pieceData)
			if err != nil {
				panic(err)
			}

			data := merkledag.NewRawNode(pieceData)

			t := time.Now()
			sectorID, err := sb.AddPiece(context.Background(), minerAddr, data.Cid(), uint64(len(pieceData)), bytes.NewReader(pieceData))
			if err != nil {
				panic(err)
			}
			fmt.Printf("Generate and add piece %s with size %d into staging sector %d, took %v\n", data.Cid(), len(pieceData), sectorID, time.Since(t))
		}
	},
}

type SimpleSectorBuilder struct {
	ptr               unsafe.Pointer
	sectorManager     *multisectorbuilder.SectorStateManager
	MetaStore         *Datastore
	MaxBytesPerSector *types.BytesAmount
}

func (sb *SimpleSectorBuilder) AddPiece(ctx context.Context, minerAddr address.Address, pieceRef cid.Cid, pieceSize uint64, reader io.Reader) (sectorID uint64, err error) {
	var staged [] multisectorbuilder.StagedSectorMetadata
	stagedMap, err := sb.sectorManager.GetStaged(minerAddr)
	if err == nil {
		for _, s := range stagedMap {
			if s.State == sealing_state.Pending {
				staged = append(staged, s)
			}
		}
	}

	sectorID, err = go_sectorbuilder.AddPieceFirst(sb.ptr, minerAddr.String(), staged, pieceSize, sb.sectorManager.GetNextSectorID(minerAddr))
	if err != nil {
		fmt.Printf("get sector id for adding piece: %s\n", err)
		return 0, err
	}
	var sector multisectorbuilder.StagedSectorMetadata
	var found bool
	for _, s := range staged {
		if sectorID == s.SectorID {
			found = true
			sector = s
			break
		}
	}
	if !found {
		sector = multisectorbuilder.StagedSectorMetadata{
			SectorID: sectorID,
			State:    sealing_state.Pending,
		}
	}

	file, err := ioutil.TempFile("", "")
	if err != nil {
		return 0, err
	}

	defer func() {
		err1 := os.Remove(file.Name())
		if err1 != nil && err == nil {
			err = err1
		}
	}()

	n, err := io.Copy(file, reader)
	if err != nil {
		return 0, err
	}

	if uint64(n) != pieceSize {
		err = fmt.Errorf("was unable to write all piece bytes to temp file (wrote %dB, pieceSize %dB)", n, pieceSize)
		return 0, err
	}

	meta, err := go_sectorbuilder.AddPieceSecond(sb.ptr, minerAddr.String(), sector, pieceRef.String(), pieceSize, file.Name())
	if err != nil {
		return 0, err
	}

	meta.UpdatedAt = time.Now()

	err = sb.sectorManager.PutStaged(minerAddr, meta)
	if err != nil {
		return 0, err
	}

	return meta.SectorID, nil
}

func (sb *SimpleSectorBuilder) Close() {
	go_sectorbuilder.DestroySimpleSectorBuilder(sb.ptr)
	sb.MetaStore.Close()
}

func (sb *SimpleSectorBuilder) SealAllStagedUnsealedSectors() {
	stagedMap, _ := sb.sectorManager.GetStaged(minerAddr) // ignore error
	var stagedSectorIDs []string
	if len(stagedMap) > 0 {
		for id := range stagedMap {
			stagedSectorIDs = append(stagedSectorIDs, fmt.Sprint(id))
		}
	}

	sealedMap, _ := sb.sectorManager.GetSealed(minerAddr) // ignore error
	var sealedSectorIDs []string
	if len(sealedMap) > 0 {
		for id := range sealedMap {
			sealedSectorIDs = append(sealedSectorIDs, fmt.Sprint(id))
		}
	}

	fmt.Println("Seal all staged sectors ...")
	fmt.Printf("  staged sectors: [%s]\n", blue(strings.Join(stagedSectorIDs, ", ")))
	fmt.Printf("  sealed sectors: [%s]\n", blue(strings.Join(sealedSectorIDs, ", ")))

	if len(stagedSectorIDs) == 0 {
		fmt.Println("No staged sector needs to seal")
		return
	}

	var wg sync.WaitGroup
	for id, stagedSector := range stagedMap {
		go func() {
			defer wg.Done()
			start := time.Now()
			sealedSector, err := go_sectorbuilder.SealStagedSector(sb.ptr, minerAddr.String(), stagedSector, sectorbuilder.AddressToProverID(minerAddr))
			if err != nil {
				fmt.Printf("Sealing %s: sector %d, took %v, error %s\n",
					red("failed"), id, time.Since(start), red(err))
				return
			}

			fmt.Printf("Sealing %s: sector %d, took %v\n", blue("succeeded"), id, time.Since(start))
			for _, pieceInfo := range sealedSector.Pieces {
				fmt.Printf("  Piece %s, size %d\n", cyan(pieceInfo.Key), pieceInfo.Size)
			}

			err = sb.sectorManager.PutSealed(minerAddr, sealedSector)
			if err != nil {
				panic(err)
			}
		}()
	}
	wg.Wait()
}

func (sb *SimpleSectorBuilder) GeneratePoSt(minerAddr address.Address, r sectorbuilder.GeneratePoStRequest) (sectorbuilder.GeneratePoStResponse, error) {
	sealedSectorsMap, err := sb.sectorManager.GetSealed(minerAddr)
	if err != nil {
		return sectorbuilder.GeneratePoStResponse{}, err
	}
	sealedSectors := make([]multisectorbuilder.SealedSectorMetadata, 0, len(r.SortedSectorInfo.Values()))
	for _, info := range r.SortedSectorInfo.Values() {
		s, ok := sealedSectorsMap[info.SectorID]
		if !ok {
			return sectorbuilder.GeneratePoStResponse{}, errors.Errorf("sealed sector %d not found", info.SectorID)
		}
		sealedSectors = append(sealedSectors, s)
	}

	var faults []uint64 // TODO: wait for implementation
	challenges, err := go_sectorbuilder.GeneratePoStFirst(sb.ptr, r.ChallengeSeed, faults, sealedSectors)
	if err != nil {
		return sectorbuilder.GeneratePoStResponse{}, err
	}

	challengedSectors := make([]multisectorbuilder.SealedSectorMetadata, 0, len(challenges))
	for _, c := range challenges {
		s, ok := sealedSectorsMap[c.Sector]
		if !ok {
			return sectorbuilder.GeneratePoStResponse{}, errors.Errorf("sealed sector %d not found", c.Sector)
		}
		challengedSectors = append(challengedSectors, s)
	}

	proof, err := go_sectorbuilder.GeneratePoStSecond(sb.ptr, minerAddr.String(), challenges, faults, challengedSectors)
	if err != nil {
		return sectorbuilder.GeneratePoStResponse{}, err
	}
	postRep := &sectorbuilder.GeneratePoStResponse{
		Proof: proof,
	}
	_ = sb.sectorManager.PutPoSt(minerAddr, &r, postRep) // ignore cache error
	return *postRep, nil
}

func openSimpleSectorBuilder() *SimpleSectorBuilder {
	ds := openMetaDatastore()

	stagingDir := filepath.Join(getFilutilDir(), "staging")
	sealedDir := filepath.Join(getFilutilDir(), "sealed")

	sectorClass := types.NewSectorClass(types.TwoHundredFiftySixMiBSectorSize)

	ptr, err := go_sectorbuilder.InitSimpleSectorBuilder(
		sectorClass.SectorSize().Uint64(),
		uint8(sectorClass.PoRepProofPartitions().Int()),
		uint8(sectorClass.PoStProofPartitions().Int()),
		sealedDir,
		stagingDir,
		sectorbuilder.MaxNumStagedSectors,
	)

	if err != nil {
		panic(err)
	}

	max := types.NewBytesAmount(go_sectorbuilder.GetMaxUserBytesPerStagedSector(sectorClass.SectorSize().Uint64()))

	return &SimpleSectorBuilder{
		ptr:               ptr,
		sectorManager:     multisectorbuilder.NewSectorStateManager(ds.Datastore),
		MetaStore:         ds,
		MaxBytesPerSector: max,
	}
}

var SimpleSectorBuilderSealSectorsCmd = &cobra.Command{
	Use:   "seal-sectors",
	Short: "Seal all staged sectors",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSimpleSectorBuilder()
		defer sb.Close()

		sb.SealAllStagedUnsealedSectors()
	},
}

var SimpleSectorBuilderVerifySectorsPorepCmd = &cobra.Command{
	Use:   "verify-sectors-porep",
	Short: "Verify PoRep (Proof-of-Replication) of all sealed sectors",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSimpleSectorBuilder()
		defer sb.Close()

		sealedMap, _ := sb.sectorManager.GetSealed(minerAddr) // ignore error
		var sealedSectorIDs []string
		if len(sealedMap) > 0 {
			for id := range sealedMap {
				sealedSectorIDs = append(sealedSectorIDs, fmt.Sprint(id))
			}
		}

		fmt.Printf("All sealed sectors: [%s]\n", blue(strings.Join(sealedSectorIDs, ", ")))
		if len(sealedMap) == 0 {
			return
		}

		for _, s := range sealedMap {
			fmt.Printf("Verify sector %d: ", s.SectorID)
			t := time.Now()
			res, err := (&verification.RustVerifier{}).VerifySeal(verification.VerifySealRequest{
				CommD:      s.CommD,
				CommR:      s.CommR,
				CommRStar:  s.CommRStar,
				Proof:      s.Proof,
				ProverID:   sectorbuilder.AddressToProverID(minerAddr),
				SectorID:   s.SectorID,
				SectorSize: types.TwoHundredFiftySixMiBSectorSize,
			})
			if err != nil {
				fmt.Printf("error %s", red(err))
			} else if !res.IsValid {
				fmt.Print(red("invalid"))
			} else {
				fmt.Print("valid")
			}
			fmt.Printf(", took %v\n", time.Since(t))
		}
	},
}

var SimpleSectorBuilderLsSectorsCmd = &cobra.Command{
	Use:   "ls-sectors",
	Short: "List all sectors",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSimpleSectorBuilder()
		defer sb.Close()

		fmt.Println(green("Staged sectors:"))
		stagedMap, _ := sb.sectorManager.GetStaged(minerAddr)
		if len(stagedMap) > 0 {
			for id := range stagedMap {
				fmt.Printf("  Sector %d\n", id)
			}
		}

		fmt.Println(green("Sealed sectors:"))
		sealedMap, _ := sb.sectorManager.GetSealed(minerAddr)
		if len(sealedMap) > 0 {
			for id, sector := range sealedMap {
				fmt.Printf("  Sector %d\n", id)
				for _, p := range sector.Pieces {
					fmt.Printf("    Piece %s, size %d\n", cyan(p.Key), p.Size)
				}
			}
		}
	},
}

var SimpleSectorBuilderVerifySectorsPostCmd = &cobra.Command{
	Use:   "verify-sectors-post",
	Short: "Challenge and verify PoSt (Proof-of-Spacetime) of all sealed sectors",
	Run: func(cmd *cobra.Command, args []string) {
		var challengeSeed types.PoStChallengeSeed
		_, err := io.ReadFull(rand.Reader, challengeSeed[:])
		if err != nil {
			panic(err)
		}
		fmt.Printf("Use challenge seed: %s\n", hex.EncodeToString(challengeSeed[:]))

		sb := openSimpleSectorBuilder()
		defer sb.Close()

		sealedMap, _ := sb.sectorManager.GetSealed(minerAddr) // ignore error
		var sectorInfos []go_sectorbuilder.SectorInfo
		var sealedSectorIDs []string
		if len(sealedMap) > 0 {
			for id, s := range sealedMap {
				sectorInfos = append(sectorInfos, go_sectorbuilder.SectorInfo{
					SectorID: s.SectorID,
					CommR:    s.CommR,
				})
				sealedSectorIDs = append(sealedSectorIDs, fmt.Sprint(id))
			}
		}
		sortedSectorInfo := go_sectorbuilder.NewSortedSectorInfo(sectorInfos...)

		fmt.Println("Generate PoSt ...")
		t := time.Now()
		gres, err := sb.GeneratePoSt(minerAddr, sectorbuilder.GeneratePoStRequest{
			SortedSectorInfo: sortedSectorInfo,
			ChallengeSeed:    challengeSeed,
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("  sectors: [%s]\n", blue(strings.Join(sealedSectorIDs, ", ")))
		fmt.Printf("  proof %s, took %v\n", hex.EncodeToString(gres.Proof), time.Since(t))

		fmt.Println("Verify PoSt ...")
		t = time.Now()
		vres, err := (&verification.RustVerifier{}).VerifyPoSt(verification.VerifyPoStRequest{
			ChallengeSeed:    challengeSeed,
			SortedSectorInfo: sortedSectorInfo,
			Faults:           []uint64{},
			Proof:            gres.Proof,
			SectorSize:       types.TwoHundredFiftySixMiBSectorSize,
		})
		if err != nil {
			panic(err)
		}
		if !vres.IsValid {
			fmt.Print(red("  invalid"))
		} else {
			fmt.Print("  valid")
		}
		fmt.Printf(", took %v\n", time.Since(t))
	},
}
