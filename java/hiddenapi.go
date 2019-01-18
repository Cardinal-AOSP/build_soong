// Copyright 2019 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package java

import (
	"sort"
	"strings"
	"sync"

	"github.com/google/blueprint"

	"android/soong/android"
)

var hiddenAPIGenerateCSVRule = pctx.AndroidStaticRule("hiddenAPIGenerateCSV", blueprint.RuleParams{
	Command:     "${config.Class2Greylist} --public-api-list ${publicAPIList} $in $outFlag $out",
	CommandDeps: []string{"${config.Class2Greylist}"},
}, "outFlag", "publicAPIList")

func hiddenAPIGenerateCSV(ctx android.ModuleContext, classesJar android.Path) {
	flagsCSV := android.PathForModuleOut(ctx, "hiddenapi", "flags.csv")
	metadataCSV := android.PathForModuleOut(ctx, "hiddenapi", "metadata.csv")
	publicList := &bootImagePath{ctx.Config().HiddenAPIPublicList()}

	ctx.Build(pctx, android.BuildParams{
		Rule:        hiddenAPIGenerateCSVRule,
		Description: "hiddenapi flags",
		Input:       classesJar,
		Output:      flagsCSV,
		Implicit:    publicList,
		Args: map[string]string{
			"outFlag":       "--write-flags-csv",
			"publicAPIList": publicList.String(),
		},
	})

	ctx.Build(pctx, android.BuildParams{
		Rule:        hiddenAPIGenerateCSVRule,
		Description: "hiddenapi metadata",
		Input:       classesJar,
		Output:      metadataCSV,
		Implicit:    publicList,
		Args: map[string]string{
			"outFlag":       "--write-metadata-csv",
			"publicAPIList": publicList.String(),
		},
	})

	hiddenAPISaveCSVOutputs(ctx, flagsCSV, metadataCSV)
}

var hiddenAPIEncodeDexRule = pctx.AndroidStaticRule("hiddenAPIEncodeDex", blueprint.RuleParams{
	Command: `rm -rf $tmpDir && mkdir -p $tmpDir && mkdir $tmpDir/dex-input && mkdir $tmpDir/dex-output && ` +
		`unzip -o -q $in 'classes*.dex' -d $tmpDir/dex-input && ` +
		`for INPUT_DEX in $$(find $tmpDir/dex-input -maxdepth 1 -name 'classes*.dex' | sort); do ` +
		`  echo "--input-dex=$${INPUT_DEX}"; ` +
		`  echo "--output-dex=$tmpDir/dex-output/$$(basename $${INPUT_DEX})"; ` +
		`done | xargs ${config.HiddenAPI} encode --api-flags=$flags && ` +
		`${config.SoongZipCmd} -o $tmpDir/dex.jar -C $tmpDir/dex-output -f "$tmpDir/dex-output/classes*.dex" && ` +
		`${config.MergeZipsCmd} -D -zipToNotStrip $tmpDir/dex.jar -stripFile "classes*.dex" $out $tmpDir/dex.jar $in`,
	CommandDeps: []string{
		"${config.HiddenAPI}",
		"${config.SoongZipCmd}",
		"${config.MergeZipsCmd}",
	},
}, "flags", "tmpDir")

func hiddenAPIEncodeDex(ctx android.ModuleContext, output android.WritablePath, dexInput android.WritablePath) {
	flags := &bootImagePath{ctx.Config().HiddenAPIFlags()}

	ctx.Build(pctx, android.BuildParams{
		Rule:        hiddenAPIEncodeDexRule,
		Description: "hiddenapi encode dex",
		Input:       dexInput,
		Output:      output,
		Implicit:    flags,
		Args: map[string]string{
			"flags":  flags.String(),
			"tmpDir": android.PathForModuleOut(ctx, "hiddenapi", "dex").String(),
		},
	})

	hiddenAPISaveDexInputs(ctx, dexInput)
}

const hiddenAPIOutputsKey = "hiddenAPIOutputsKey"

var hiddenAPIOutputsLock sync.Mutex

func hiddenAPIGetOutputs(config android.Config) (*android.Paths, *android.Paths, *android.Paths) {
	type threePathsPtrs [3]*android.Paths
	s := config.Once(hiddenAPIOutputsKey, func() interface{} {
		return threePathsPtrs{new(android.Paths), new(android.Paths), new(android.Paths)}
	}).(threePathsPtrs)
	return s[0], s[1], s[2]
}

func hiddenAPISaveCSVOutputs(ctx android.ModuleContext, flagsCSV, metadataCSV android.Path) {
	flagsCSVList, metadataCSVList, _ := hiddenAPIGetOutputs(ctx.Config())

	hiddenAPIOutputsLock.Lock()
	defer hiddenAPIOutputsLock.Unlock()

	*flagsCSVList = append(*flagsCSVList, flagsCSV)
	*metadataCSVList = append(*metadataCSVList, metadataCSV)
}

func hiddenAPISaveDexInputs(ctx android.ModuleContext, dexInput android.Path) {
	_, _, dexInputList := hiddenAPIGetOutputs(ctx.Config())

	hiddenAPIOutputsLock.Lock()
	defer hiddenAPIOutputsLock.Unlock()

	*dexInputList = append(*dexInputList, dexInput)
}

func init() {
	android.RegisterMakeVarsProvider(pctx, hiddenAPIMakeVars)
}

func hiddenAPIMakeVars(ctx android.MakeVarsContext) {
	flagsCSVList, metadataCSVList, dexInputList := hiddenAPIGetOutputs(ctx.Config())

	export := func(name string, paths *android.Paths) {
		s := paths.Strings()
		sort.Strings(s)
		ctx.Strict(name, strings.Join(s, " "))
	}

	export("SOONG_HIDDENAPI_FLAGS", flagsCSVList)
	export("SOONG_HIDDENAPI_GREYLIST_METADATA", metadataCSVList)
	export("SOONG_HIDDENAPI_DEX_INPUTS", dexInputList)
}
