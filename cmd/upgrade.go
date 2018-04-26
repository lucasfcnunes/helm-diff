package cmd

import (
	"os"

	"github.com/databus23/helm-diff/diff"
	"github.com/databus23/helm-diff/manifest"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"
)

type diffCmd struct {
	release         string
	chart           string
	chartVersion    string
	client          helm.Interface
	valueFiles      valueFiles
	values          []string
	reuseValues     bool
	resetValues     bool
	suppressedKinds []string
}

const globalUsage = `Show a diff explaining what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a chart plus values.
This can be used visualize what changes a helm upgrade will
perform.
`

func newChartCommand() *cobra.Command {
	diff := diffCmd{}

	cmd := &cobra.Command{
		Use:     "upgrade [flags] [RELEASE] [CHART]",
		Short:   "Show a diff explaining what a helm upgrade would change.",
		Long:    globalUsage,
		Example: "helm diff upgrade my-release stable/postgresql --values values.yaml",
		Args: func(cmd *cobra.Command, args []string) error {
			return checkArgsLength(len(args), "release name", "chart path")
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

			diff.release = args[0]
			diff.chart = args[1]
			if diff.client == nil {
				diff.client = helm.NewClient(helm.Host(os.Getenv("TILLER_HOST")), helm.ConnectTimeout(int64(30)))
			}
			return diff.run()
		},
	}

	f := cmd.Flags()
	f.StringVar(&diff.chartVersion, "version", "", "specify the exact chart version to use. If this is not specified, the latest version is used")
	f.BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	f.VarP(&diff.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	f.StringArrayVar(&diff.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&diff.reuseValues, "reuse-values", false, "reuse the last release's values and merge in any new values")
	f.BoolVar(&diff.resetValues, "reset-values", false, "reset the values to the ones built into the chart and merge in any new values")
	f.StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")

	return cmd

}

func (d *diffCmd) run() error {
	chartPath, err := locateChartPath(d.chart, d.chartVersion, false, "")
	if err != nil {
		return err
	}

	if err := d.valueFiles.Valid(); err != nil {
		return err
	}

	rawVals, err := d.vals()
	if err != nil {
		return err
	}

	releaseResponse, err := d.client.ReleaseContent(d.release)

	if err != nil {
		return prettyError(err)
	}

	upgradeResponse, err := d.client.UpdateRelease(
		d.release,
		chartPath,
		helm.UpdateValueOverrides(rawVals),
		helm.ReuseValues(d.reuseValues),
		helm.ResetValues(d.resetValues),
		helm.UpgradeDryRun(true),
	)
	if err != nil {
		return prettyError(err)
	}

	currentSpecs := manifest.Parse(releaseResponse.Release.Manifest)
	newSpecs := manifest.Parse(upgradeResponse.Release.Manifest)

	diff.DiffManifests(currentSpecs, newSpecs, d.suppressedKinds, os.Stdout)

	return nil
}
