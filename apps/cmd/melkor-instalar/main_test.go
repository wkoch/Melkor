package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"melkor.local/apps/internal/pcdisk"
)

func TestSelecionaSomenteDiscoConfirmadoPeloID(t *testing.T) {
	disks := []pcdisk.Disk{{ID: "nvme-segundo", Bytes: 13 * pcdisk.GiB, Sector: 512},
		{ID: "nvme-principal", Model: "SSD", Bytes: 16 * pcdisk.GiB, Sector: 512}}
	for _, test := range []struct {
		input string
		want  string
	}{
		{"nvme-principal\nAPAGAR nvme-principal\n", "nvme-principal"},
		{"nvme-segundo\nAPAGAR nvme-principal\n", ""},
		{"desconhecido\n", ""},
		{"nvme-principal\n", ""},
		{"", ""},
	} {
		var output bytes.Buffer
		chosen, err := selectDisk(bufio.NewReader(strings.NewReader(test.input)), &output, disks)
		if chosen.ID != test.want || (err == nil) != (test.want != "") {
			t.Fatalf("id=%q, erro=%v", chosen.ID, err)
		}
		if !strings.Contains(output.String(), "todas as partições") || !strings.Contains(output.String(), "16.0 GiB") {
			t.Fatal("a seleção precisa informar o efeito e a capacidade")
		}
	}
	if _, err := selectDisk(bufio.NewReader(strings.NewReader("")), io.Discard, nil); err == nil {
		t.Fatal("seleção aceitou lista vazia")
	}
}

func TestLeituraDeSenhaRejeitaEntradaIncompletaOuDivergente(t *testing.T) {
	for _, input := range []string{"fraca\nfraca\n", "senha-de-teste\ndiferente\n", "", "senha-de-teste\n",
		"senha-de-\tteste\nsenha-de-\tteste\n", strings.Repeat("a", 129) + "\n" + strings.Repeat("a", 129) + "\n"} {
		if _, err := readPassword(bufio.NewReader(strings.NewReader(input)), io.Discard, "root"); err == nil {
			t.Fatalf("aceitou entrada inválida")
		}
	}
	var output bytes.Buffer
	password, err := readPassword(bufio.NewReader(strings.NewReader("senha-de-teste\nsenha-de-teste\n")), &output, "root")
	if err != nil || password != "senha-de-teste" || strings.Contains(output.String(), password) {
		t.Fatal("leitura falhou ou exibiu a senha")
	}
}

func TestProvisaoExigeSenhasDistintasEUsaApenasOsHashes(t *testing.T) {
	root, user := "senha-de-root-escolhida", "senha-da-conta-comum"
	input := root + "\n" + root + "\n" + user + "\n" + user + "\n"
	want := "root:hash-root\nmelkor:hash-usuario\n"
	digest := func(password string) (string, error) {
		if password == root {
			return "hash-root", nil
		}
		if password == user {
			return "hash-usuario", nil
		}
		t.Fatal("senha inesperada")
		return "", nil
	}
	hashes, err := collectCredentials(bufio.NewReader(strings.NewReader(input)), io.Discard, digest)
	if err != nil || hashes != want {
		t.Fatal("credenciais divergentes")
	}
	failure := errors.New("hash indisponível")
	_, err = collectCredentials(bufio.NewReader(strings.NewReader(input)), io.Discard, func(string) (string, error) {
		return "", failure
	})
	if !errors.Is(err, failure) {
		t.Fatal("a falha de hashing não foi propagada")
	}
	for _, invalid := range []string{"", root + "\n" + root + "\n", strings.Repeat(root+"\n", 4)} {
		if _, err := collectCredentials(bufio.NewReader(strings.NewReader(invalid)), io.Discard, digest); err == nil {
			t.Fatal("credenciais incompletas ou repetidas foram aceitas")
		}
	}
}
