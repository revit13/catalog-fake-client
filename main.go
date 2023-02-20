package main

import (
	"emperror.dev/errors"
	"encoding/json"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"os"
	"strings"

	dcclient "fybrik.io/fybrik/pkg/connectors/datacatalog/clients"
	"fybrik.io/fybrik/pkg/logging"
	"fybrik.io/fybrik/pkg/model/datacatalog"
	"fybrik.io/fybrik/pkg/taxonomy/validate"
	"github.com/rs/zerolog"
)

var version string

const (
	requestJsonOption         = "request"
	requestOperationOption    = "operation"
	credentialPathOption      = "creds"
	catalogconnectorUrlOption = "url"
	datasetIDOption           = "datasetID"
)

var (
	requestFile         string
	requestOperation    string
	credentialPath      string
	catalogconnectorUrl string
	datasetID           string
)

type Request struct {
	log zerolog.Logger
	operationType string
}

var request Request

var DataCatalogTaxonomy = "resources/taxonomy/datacatalog.json#/definitions/GetAssetResponse"

func newDataCatalog() (dcclient.DataCatalog, error) {
	providerName := "egeria"
	return dcclient.NewDataCatalog(
		providerName,
		catalogconnectorUrl)
}

func ValidateAssetResponse(response *datacatalog.GetAssetResponse, log *zerolog.Logger) error {
	var allErrs []*field.Error
	taxonomyFile := DataCatalogTaxonomy

	// Convert GetAssetRequest Go struct to JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return err
	}
	log.Info().Msg("responseJSON:" + string(responseJSON))

	// Validate Fybrik module against taxonomy
	allErrs, err = validate.TaxonomyCheck(responseJSON, taxonomyFile)
	if err != nil {
		return err
	}

	// Return any error
	if len(allErrs) == 0 {
		return nil
	}

	return errors.New("all Err is not null")
}

func handleRead(log *zerolog.Logger) error {
	// Initialize DataCatalog interface
	catalog, err := newDataCatalog()
	if err != nil {
		return errors.Wrap(err, "unable to create data catalog facade")
	}
	defer catalog.Close()

	// Open our jsonFile
	jsonFile, err := os.Open(requestFile)
	// if we os.Open returns an error then handle it
	if err != nil {
		return errors.Wrap(err, "error opening "+requestFile)
	}
	log.Info().Msg("Successfully Opened " + requestFile)
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var dataCatalogReq datacatalog.GetAssetRequest
	json.Unmarshal(byteValue, &dataCatalogReq)
	var response *datacatalog.GetAssetResponse

	if response, err = catalog.GetAssetInfo(&dataCatalogReq, credentialPath); err != nil {
		return errors.Wrap(err, "failed to receive the catalog connector response")
	}
	err = ValidateAssetResponse(response, log)
	if err != nil {
		return errors.Wrap(err, "failed to validate the catalog connector response")
	}
	log.Info().Msg("RESPONSE VALIDATION PASS")
	return nil

}

// RootCmd defines the root cli command
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "catalog-connector-client",
		Short:         "Data catalog connector client",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       strings.TrimSpace(version),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := request.log.With().Str(logging.DATASETID, datasetID).Logger()
			if requestOperation == "read" {
				request.operationType = "read"
				return handleRead(&log)
			}
			return errors.New("Unsupported operation")
		},
	}
	cmd.PersistentFlags().StringVar(&requestFile, requestJsonOption, "resources/read-request.json", "Json file containing the data catalog request")
	cmd.PersistentFlags().StringVar(&requestOperation, requestOperationOption, "read", "Request operation")
	cmd.PersistentFlags().StringVar(&credentialPath, credentialPathOption, "/v1/kubernetes-secrets/my-secret?namespace=default", "Credential path")
	cmd.PersistentFlags().StringVar(&catalogconnectorUrl, catalogconnectorUrlOption, "http://localhost:8888", "Catalog connector Url")
	cmd.PersistentFlags().StringVar(&datasetID, datasetIDOption, "demo-dataset", "Dataset ID")
	cmd.MarkFlagsRequiredTogether(requestJsonOption, requestOperationOption, credentialPathOption, catalogconnectorUrlOption, datasetIDOption)

	return cmd
}

func main() {
	request.log = logging.LogInit(logging.CONTROLLER, "DataCatalogConnectorClient")
	if err := RootCmd().Execute(); err != nil {
		request.log.Error().Err(err).Msg("request failed")
		os.Exit(1)
	}

}
