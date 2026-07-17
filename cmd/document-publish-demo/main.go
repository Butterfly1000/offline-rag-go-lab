package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/documentingest"
	"offline-rag-go-lab/internal/fileconfig"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local project config")
	mode := flag.String("mode", "resolve", "publish, rollback, or resolve")
	scope := flag.String("scope", "document-ingestion-course", "snapshot knowledge scope")
	collection := flag.String("collection", "", "publish target collection")
	from := flag.String("from", "", "expected current alias collection")
	to := flag.String("to", "", "rollback target collection")
	flag.Parse()
	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	required := func(key string) string {
		value, err := fileconfig.Required(values, key)
		if err != nil {
			log.Fatal(err)
		}
		return value
	}
	alias := required("DOCUMENT_INGEST_ALIAS")
	v1, v2 := required("DOCUMENT_INGEST_COLLECTION_V1"), required("DOCUMENT_INGEST_COLLECTION_V2")
	allowed := map[string]bool{v1: true, v2: true, "": true}
	if !allowed[strings.TrimSpace(*collection)] || !allowed[strings.TrimSpace(*from)] || !allowed[strings.TrimSpace(*to)] {
		log.Fatal("collection values must use configured v1/v2 snapshots")
	}
	db, err := sql.Open("mysql", required("RECENT_CHAT_MYSQL_DSN"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	store := documentingest.NewMySQLManifestStore(db)
	index := documentingest.NewQdrantIndex(required("QDRANT_BASE_URL"))
	publisher := documentingest.Publisher{Index: index, Store: store, MaxVerificationAge: time.Minute}
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "publish":
		snapshot, err := store.LoadReadySnapshot(ctx, *scope, *collection)
		if err != nil {
			log.Fatal(err)
		}
		report, err := publisher.Verify(ctx, documentingest.VerifyRequest{Snapshot: snapshot, ExpectedVectorSize: 1024})
		if err != nil {
			log.Fatal(err)
		}
		result, err := publisher.Activate(ctx, documentingest.ActivateRequest{Alias: alias, From: *from, Snapshot: snapshot, Verification: report})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Verified collection: %s\nManifest digest: %s\nPoints: %d\nAlias: %s\nFrom: %s\nTo: %s\nAlias switched: %t\nMySQL activated: %t\n", report.Collection, report.ManifestDigest, report.PointCount, alias, result.From, result.To, result.AliasSwitched, result.MySQLActivated)
	case "rollback":
		snapshot, err := store.LoadReadySnapshot(ctx, *scope, *to)
		if err != nil {
			log.Fatal(err)
		}
		result, err := publisher.Rollback(ctx, documentingest.RollbackRequest{Alias: alias, From: *from, Snapshot: snapshot})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Alias: %s\nRolled back from: %s\nRolled back to: %s\nAlias switched: %t\nMySQL activated: %t\nCollections deleted: 0\n", alias, result.From, result.To, result.AliasSwitched, result.MySQLActivated)
	case "resolve":
		target, err := index.ResolveAlias(ctx, alias)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Alias: %s\nTarget: %s\n", alias, target)
	default:
		log.Fatalf("unsupported --mode %q", *mode)
	}
}
