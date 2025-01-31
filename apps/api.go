package apps

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	api "github.com/lnbits/lnbits/api"
	models "github.com/lnbits/lnbits/models"
	"github.com/lnbits/lnbits/storage"
)

func Info(w http.ResponseWriter, r *http.Request) {
	app := appidToURL(mux.Vars(r)["appid"])
	_, settings, err := getAppSettings(app)
	if err != nil {
		api.SendJSONError(w, 400, "failed to get app settings: %s", err.Error())
		return
	}

	json.NewEncoder(w).Encode(settings)
}

func ListItems(w http.ResponseWriter, r *http.Request) {
	app := appidToURL(mux.Vars(r)["appid"])
	wallet := r.Context().Value("wallet").(*models.Wallet)
	modelName := mux.Vars(r)["model"]

	_, settings, err := getAppSettings(app)
	if err != nil {
		api.SendJSONError(w, 400, "failed to get app settings: %s", err.Error())
		return
	}

	var items []models.AppDataItem
	result := storage.DB.
		Where(&models.AppDataItem{WalletID: wallet.ID, App: app, Model: modelName}).
		Find(&items)

	if result.Error != nil {
		api.SendJSONError(w, 500, "database error: %s", result.Error.Error())
		return
	}

	// preprocess items
	/// computed
	model := settings.getModel(modelName)
	for _, field := range model.Fields {
		if field.Computed != nil {
			for _, item := range items {
				item.Value[field.Name], _ = runlua(RunluaParams{
					AppID: app,
					FunctionToRun: fmt.Sprintf(
						"get_model_field('%s', '%s').computed(item)",
						model.Name, field.Name,
					),
					InjectedGlobals: &map[string]interface{}{"item": item.Value},
				})
			}
		}
	}
	/// filter
	if model.Filter != nil {
		filteredItems := make([]models.AppDataItem, 0, len(items))
		for _, item := range items {
			returnedValue, _ := runlua(RunluaParams{
				AppID:           app,
				FunctionToRun:   fmt.Sprintf("get_model('%s').filter(item)", model.Name),
				InjectedGlobals: &map[string]interface{}{"item": item.Value},
			})

			if shouldKeep, ok := returnedValue.(bool); ok && shouldKeep {
				filteredItems = append(filteredItems, item)
			}
		}
		items = filteredItems
	}

	json.NewEncoder(w).Encode(items)
}

func GetItem(w http.ResponseWriter, r *http.Request) {
	app := appidToURL(mux.Vars(r)["appid"])
	model := mux.Vars(r)["model"]
	key := mux.Vars(r)["key"]
	wallet := r.Context().Value("wallet").(*models.Wallet)

	if value, err := DBGet(wallet.ID, app, model, key); err != nil {
		api.SendJSONError(w, 500, "failed to get item: %s", err.Error())
		return
	} else {
		json.NewEncoder(w).Encode(value)
	}
}

func SetItem(w http.ResponseWriter, r *http.Request) {
	app := appidToURL(mux.Vars(r)["appid"])
	model := mux.Vars(r)["model"]
	key := mux.Vars(r)["key"]
	wallet := r.Context().Value("wallet").(*models.Wallet)

	var value map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
		api.SendJSONError(w, 400, "failed to read data: %s", err.Error())
		return
	}

	if err := DBSet(wallet.ID, app, model, key, value); err != nil {
		api.SendJSONError(w, 500, "failed to set item: %s", err.Error())
		return
	}
}

func AddItem(w http.ResponseWriter, r *http.Request) {
	app := appidToURL(mux.Vars(r)["appid"])
	model := mux.Vars(r)["model"]
	wallet := r.Context().Value("wallet").(*models.Wallet)

	var value map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
		api.SendJSONError(w, 400, "failed to read data: %s", err.Error())
		return
	}

	if err := DBAdd(wallet.ID, app, model, value); err != nil {
		api.SendJSONError(w, 500, "failed to add item: %s", err.Error())
		return
	}
}

func DeleteItem(w http.ResponseWriter, r *http.Request) {
	app := appidToURL(mux.Vars(r)["appid"])
	model := mux.Vars(r)["model"]
	key := mux.Vars(r)["key"]
	wallet := r.Context().Value("wallet").(*models.Wallet)

	if err := DBDelete(wallet.ID, app, model, key); err != nil {
		api.SendJSONError(w, 500, "failed to delete item: %s", err.Error())
		return
	}
}

func CustomAction(w http.ResponseWriter, r *http.Request) {
	walletID := mux.Vars(r)["wallet"]
	app := appidToURL(mux.Vars(r)["appid"])
	action := mux.Vars(r)["action"]

	var params interface{}
	json.NewDecoder(r.Body).Decode(&params)

	_, settings, err := getAppSettings(app)
	if err != nil {
		api.SendJSONError(w, 400, "failed to get app settings: %s", err.Error())
		return
	}

	if _, ok := settings.Actions[action]; !ok {
		api.SendJSONError(w, 404, "action '%s' not defined on app: %s", action, err.Error())
		return
	}

	returned, err := runlua(RunluaParams{
		AppID:           app,
		FunctionToRun:   fmt.Sprintf("actions.%s(params)", action),
		InjectedGlobals: &map[string]interface{}{"params": params},
		WalletID:        walletID,
	})
	if err != nil {
		api.SendJSONError(w, 470, "failed to run action: %s", err.Error())
		return
	}

	json.NewEncoder(w).Encode(returned)
}

func StaticFile(w http.ResponseWriter, r *http.Request) {

}
