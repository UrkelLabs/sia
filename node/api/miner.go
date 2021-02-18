package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"gitlab.com/NebulousLabs/Sia/types"
	"gitlab.com/NebulousLabs/encoding"
)

type (
	// MinerGET contains the information that is returned after a GET request
	// to /miner.
	MinerGET struct {
		BlocksMined      int  `json:"blocksmined"`
		CPUHashrate      int  `json:"cpuhashrate"`
		CPUMining        bool `json:"cpumining"`
		StaleBlocksMined int  `json:"staleblocksmined"`
	}
)

// minerHandler handles the API call that queries the miner's status.
func (api *API) minerHandler(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	blocksMined, staleMined := api.miner.BlocksMined()
	mg := MinerGET{
		BlocksMined:      blocksMined,
		CPUHashrate:      api.miner.CPUHashrate(),
		CPUMining:        api.miner.CPUMining(),
		StaleBlocksMined: staleMined,
	}
	WriteJSON(w, mg)
}

// minerStartHandler handles the API call that starts the miner.
func (api *API) minerStartHandler(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	api.miner.StartCPUMining()
	WriteSuccess(w)
}

// minerStopHandler handles the API call to stop the miner.
func (api *API) minerStopHandler(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	api.miner.StopCPUMining()
	WriteSuccess(w)
}

// minerHeaderHandlerGET handles the API call that retrieves a block header
// for work.
func (api *API) minerHeaderHandlerGET(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	bhfw, target, err := api.miner.HeaderForWork()
	if err != nil {
		WriteError(w, Error{err.Error()}, http.StatusBadRequest)
		return
	}
	w.Write(encoding.MarshalAll(target, bhfw))
}

// minerHeaderHandlerPOST handles the API call to submit a block header to the
// miner.
func (api *API) minerHeaderHandlerPOST(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var bh types.BlockHeader
	err := encoding.NewDecoder(req.Body, encoding.DefaultAllocLimit).Decode(&bh)
	if err != nil {
		WriteError(w, Error{err.Error()}, http.StatusBadRequest)
		return
	}
	err = api.miner.SubmitHeader(bh)
	if err != nil {
		WriteError(w, Error{err.Error()}, http.StatusBadRequest)
		return
	}
	WriteSuccess(w)
}

func (api *API) minerBlockHandlerGET(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	b := api.miner.BlockTemplate()
	// if err != nil {
	// 	WriteError(w, Error{err.Error()}, http.StatusBadRequest)
	// 	return
	// }
	// w.Write(encoding.MarshalAll(b))
    WriteJSON(w, b);
}

// minerBlockHandlerPOST handles the API call to submit a solved block to the
// miner.
func (api *API) minerBlockHandlerPOST(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var b types.Block
	err := encoding.NewDecoder(req.Body, encoding.DefaultAllocLimit).Decode(&b)
	if err != nil {
		WriteError(w, Error{err.Error()}, http.StatusBadRequest)
		return
	}
	err = api.miner.SubmitBlock(b)
	if err != nil {
		WriteError(w, Error{err.Error()}, http.StatusBadRequest)
		return
	}
	WriteSuccess(w)
}
