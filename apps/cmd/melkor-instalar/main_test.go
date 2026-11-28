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

func TestAguardaComandoInstalarCore(t *testing.T) {
	for _, test := range []struct {
		input string
		ok    bool
	}{
		{"instalar core\n", true},
		{"APAGAR\ninstalar pacote noctalia\ninstalar core\r\n", true},
		{"instalar core extra\ninstalar core", false},
		{"", false},
	} {
		var output bytes.Buffer
		err := waitForInstallCommand(bufio.NewReader(strings.NewReader(test.input)), &output)
		if (err == nil) != test.ok || !strings.Contains(output.String(), "Digite instalar core") {
			t.Fatalf("entrada %q: saída %q, erro %v", test.input, &output, err)
		}
	}
}

func TestSelecionaSomenteDiscoConfirmadoPeloNumero(t *testing.T) {
	disks := []pcdisk.Disk{{ID: "nvme-segundo", Bytes: 13 * pcdisk.GiB, Sector: 512},
		{ID: "nvme-principal", Model: "SSD", Bytes: 16 * pcdisk.GiB, Sector: 512}}
	for _, test := range []struct {
		input string
		want  string
	}{
		{"2\nAPAGAR\n", "nvme-principal"},
		{"1\r\nAPAGAR\r\n", "nvme-segundo"},
		{"1\nAPAGAR 2\n", ""},
		{"1\n\n", ""},
		{"\nAPAGAR\n", ""},
		{"0\nAPAGAR\n", ""},
		{"-1\nAPAGAR\n", ""},
		{"3\nAPAGAR\n", ""},
		{"desconhecido\n", ""},
		{"2\n", ""},
		{"2\nAPAGAR", ""},
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
		if test.want != "" && (!strings.Contains(output.String(), test.want) ||
			!strings.Contains(output.String(), "Digite APAGAR para confirmar: ")) {
			t.Fatal("a confirmação precisa identificar a unidade selecionada")
		}
	}
	if _, err := selectDisk(bufio.NewReader(strings.NewReader("")), io.Discard, nil); err == nil {
		t.Fatal("seleção aceitou lista vazia")
	}
}

func TestLeituraDeSenhaRejeitaEntradaIncompletaOuDivergente(t *testing.T) {
	for _, input := range []string{"fraca\nfraca\n", "senha-de-teste\ndiferente\n", "", "senha-de-teste\n",
		"senha-de-\tteste\nsenha-de-\tteste\n", strings.Repeat("a", 129) + "\n" + strings.Repeat("a", 129) + "\n"} {
		if _, err := readPassword(bufio.NewReader(strings.NewReader(input)), io.Discard, "root", ""); err == nil {
			t.Fatalf("aceitou entrada inválida")
		}
	}
	var output bytes.Buffer
	password, err := readPassword(bufio.NewReader(strings.NewReader("senha-de-teste\nsenha-de-teste\n")), &output, "root", "")
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

func TestLeituraDeSenhaRepeteComMotivoSemAbortarInstalacao(t *testing.T) {
	root := "senha-de-root-escolhida"
	for _, test := range []struct {
		name, input, forbidden, message string
	}{
		{"curta", "curta\ncurta\n" + root + "\n" + root + "\n", "", "precisa ter de 12 a 128 bytes"},
		{"divergente", "senha-de-teste\noutra-senha-x\n" + root + "\n" + root + "\n", "", "não conferem"},
		{"igual ao root", root + "\n" + root + "\n" + root + "\n" + root + "\n" + "conta-comum-x\nconta-comum-x\n", root, "diferente da de root"},
		{"enter extra", "\n" + "\n" + root + "\n" + root + "\n", "", "precisa ter de 12 a 128 bytes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			password, err := readPassword(bufio.NewReader(strings.NewReader(test.input)), &output, "melkor", test.forbidden)
			if err != nil {
				t.Fatalf("a repetição deveria obter uma senha válida: %v", err)
			}
			if password != root && password != "conta-comum-x" {
				t.Fatalf("senha inesperada")
			}
			if !strings.Contains(output.String(), test.message) {
				t.Fatalf("mensagem ausente %q na saída %q", test.message, output.String())
			}
			if strings.Contains(output.String(), root) || strings.Contains(output.String(), "conta-comum-x") {
				t.Fatal("a repetição exibiu a senha")
			}
		})
	}
	var output bytes.Buffer
	if _, err := readPassword(bufio.NewReader(strings.NewReader("curta\ncurta\n")), &output, "root", ""); err == nil {
		t.Fatal("entrada sem senha válida deveria terminar com erro ao chegar ao fim")
	}
}
