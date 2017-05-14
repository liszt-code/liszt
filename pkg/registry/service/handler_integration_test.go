package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/bsdlp/apiutils"
	"github.com/jmoiron/sqlx"
	"github.com/liszt-code/liszt/migrations"
	"github.com/liszt-code/liszt/pkg/registry"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

type handlerIntegrationTestObject struct {
	databaseName string
	db           *sqlx.DB
	svc          *Service
	server       *httptest.Server
}

func newHandlerIntegrationTestObject(t *testing.T) (hito *handlerIntegrationTestObject) {
	db, err := sqlx.Open("mysql", "root:@/")
	if err != nil {
		t.Fatal(err)
	}

	testDatabaseName := "test" + strconv.FormatInt(time.Now().Unix(), 10)

	_, err = db.Exec("create database " + testDatabaseName)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("use " + testDatabaseName)
	if err != nil {
		t.Fatal(err)
	}

	err = migrations.Migrate(db.DB)
	if err != nil {
		t.Fatal(err)
	}

	svc := &Service{
		Registrar: &registry.MySQLRegistrar{
			DB: db,
		},
	}
	hito = &handlerIntegrationTestObject{
		databaseName: testDatabaseName,
		db:           db,
		svc:          svc,
		server:       httptest.NewServer(svc),
	}
	return
}

func (hito *handlerIntegrationTestObject) teardown(t *testing.T) {
	_, err := hito.db.Exec("drop database " + hito.databaseName)
	assert.NoError(t, err)
	assert.NoError(t, hito.db.Close())
	hito.server.Close()
}

func TestIntegrationHandler(t *testing.T) {
	t.Run("ListUnitResidentsHandler", func(t *testing.T) {
		hito := newHandlerIntegrationTestObject(t)
		defer hito.teardown(t)

		existingUnitName := uuid.NewV4().String()
		registeredUnit, err := hito.svc.Registrar.RegisterUnit(context.Background(), &registry.Unit{Name: existingUnitName})
		if err != nil {
			t.Fatal(err)
		}
		registeredUnitID := strconv.FormatInt(registeredUnit.ID, 10)

		expectedResidents := make([]*registry.Resident, 4)
		for i := range expectedResidents {
			expectedResidents[i], err = hito.svc.Registrar.RegisterResident(context.Background(), &registry.Resident{})
			if err != nil {
				t.Fatal(err)
			}
			err = hito.svc.Registrar.MoveResident(context.Background(), expectedResidents[i].ID, registeredUnit.ID)
			if err != nil {
				t.Fatal(err)
			}
		}

		t.Run("success", func(t *testing.T) {
			assert := assert.New(t)
			resp, err := http.Get(hito.server.URL + "/units/residents?unit_id=" + registeredUnitID)
			assert.NoError(err)
			defer func() {
				assert.NoError(resp.Body.Close())
			}()
			assert.Equal(http.StatusOK, resp.StatusCode)

			var residents []*registry.Resident
			err = json.NewDecoder(resp.Body).Decode(&residents)
			assert.NoError(err)
			assert.Equal(expectedResidents, residents)
		})

		t.Run("unit not found", func(t *testing.T) {
			assert := assert.New(t)
			resp, err := http.Get(hito.server.URL + "/units/residents?unit_id=1234")
			assert.NoError(err)
			defer func() {
				assert.NoError(resp.Body.Close())
			}()
			assert.Equal(http.StatusNotFound, resp.StatusCode)

			var errObj apiutils.ErrorObject
			err = json.NewDecoder(resp.Body).Decode(&errObj)
			assert.NoError(err)
			assert.Equal(apiutils.ErrNotFound, errObj)
		})
	})

	t.Run("GetUnitByNameHandler", func(t *testing.T) {
		hito := newHandlerIntegrationTestObject(t)
		defer hito.teardown(t)
		existingUnitName := uuid.NewV4().String()
		registeredUnit, err := hito.svc.Registrar.RegisterUnit(context.Background(), &registry.Unit{Name: existingUnitName})
		if err != nil {
			t.Fatal(err)
		}
		t.Run("success", func(t *testing.T) {
			assert := assert.New(t)
			resp, err := http.Get(hito.server.URL + "/units?unit=" + existingUnitName)
			assert.NoError(err)
			assert.Equal(http.StatusOK, resp.StatusCode)
			defer func() {
				assert.NoError(resp.Body.Close())
			}()

			retrievedUnit := new(registry.Unit)
			err = json.NewDecoder(resp.Body).Decode(retrievedUnit)
			assert.NoError(err)
			assert.Equal(registeredUnit, retrievedUnit)
		})
		t.Run("unit not found", func(t *testing.T) {
			assert := assert.New(t)
			resp, err := http.Get(hito.server.URL + "/units?unit=" + uuid.NewV4().String())
			assert.NoError(err)
			assert.Equal(http.StatusNotFound, resp.StatusCode)
			defer func() {
				assert.NoError(resp.Body.Close())
			}()

			var errObj apiutils.ErrorObject
			err = json.NewDecoder(resp.Body).Decode(&errObj)
			assert.NoError(err)
			assert.Equal(apiutils.ErrNotFound, errObj)
		})
	})

	t.Run("RegisterResidentHandler", func(t *testing.T) {
		assert := assert.New(t)
		hito := newHandlerIntegrationTestObject(t)
		defer hito.teardown(t)

		resident := &registry.Resident{
			Firstname:  "Josiah",
			Middlename: "Edward",
			Lastname:   "Bartlet",
		}

		var bs bytes.Buffer
		err := json.NewEncoder(&bs).Encode(resident)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(hito.server.URL+"/residents/register", "application/json", &bs)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			closeErr := resp.Body.Close()
			assert.NoError(closeErr)
		}()
		registeredResident := new(registry.Resident)
		err = json.NewDecoder(resp.Body).Decode(registeredResident)
		assert.NoError(err)
		assert.NotEmpty(registeredResident)

		rows, err := hito.db.Queryx("select * from residents where residents.id = ?", registeredResident.ID)
		assert.NoError(err)
		defer func() {
			closeErr := rows.Close()
			assert.NoError(closeErr)
		}()

		storedResidents := []*registry.Resident{}
		for rows.Next() {
			resident := new(registry.Resident)
			err = rows.StructScan(resident)
			if err != nil {
				assert.NoError(err)
			}
			storedResidents = append(storedResidents, resident)
		}
		assert.NoError(rows.Err())
		assert.Len(storedResidents, 1)
		assert.Equal(storedResidents[0], registeredResident)
	})

	t.Run("MoveResidentHandler", func(t *testing.T) {
		hito := newHandlerIntegrationTestObject(t)
		defer hito.teardown(t)
	})

	t.Run("DeregisterResidentHandler", func(t *testing.T) {
		hito := newHandlerIntegrationTestObject(t)
		defer hito.teardown(t)
	})
}
