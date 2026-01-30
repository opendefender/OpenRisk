package handlers

import (
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// ExportRisksPDF gnre un rapport PDF de tous les risques actifs.
func ExportRisksPDF(c fiber.Ctx) error {
	var risks []domain.Risk

	// . Rcuprer les donnes
	if err := database.DB.Preload("Assets").Find(&risks).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "Failed to fetch risks for export"})
	}

	// . Initialisation du PDF
	pdf := fpdf.New("P", "mm", "A", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", )
	pdf.SetFillColor(, , ) // Gris clair

	// . Titre et Mtadonnes
	pdf.CellFormat(, , "OpenRisk - Rapport d'Évaluation des Risques", "", , "C", false, , "")
	pdf.SetFont("Arial", "", )
	pdf.CellFormat(, , fmt.Sprintf("Date du rapport: %s", time.Now().Format(" Jan ")), "", , "C", false, , "")
	pdf.Ln()

	// . En-tête du tableau
	header := []string{"Score", "Titre du Risque", "Impact", "Proba", "Assets Impacts"}
	colWidths := []float{, , , , }

	pdf.SetFont("Arial", "B", )
	for i, h := range header {
		pdf.CellFormat(colWidths[i], , h, "", , "C", true, , "")
	}
	pdf.Ln(-)

	// . Contenu du tableau
	pdf.SetFont("Arial", "", )
	for _, risk := range risks {
		// Assets list (simple string)
		assetNames := ""
		for i, asset := range risk.Assets {
			assetNames += asset.Name
			if i < len(risk.Assets)- {
				assetNames += ", "
			}
		}

		// Pour la lisibilit, les cellules sont dfinies par ligne
		pdf.CellFormat(colWidths[], , fmt.Sprintf("%.f", risk.Score), "", , "C", false, , "")
		pdf.CellFormat(colWidths[], , risk.Title, "", , "L", false, , "")
		pdf.CellFormat(colWidths[], , fmt.Sprintf("%d", risk.Impact), "", , "C", false, , "")
		pdf.CellFormat(colWidths[], , fmt.Sprintf("%d", risk.Probability), "", , "C", false, , "")

		// Grer le cas où le texte des assets est trop long (multi-line)
		x, y := pdf.GetXY()
		pdf.MultiCell(colWidths[], , assetNames, "", "L", false)
		pdf.SetXY(x+colWidths[], y) // Repositionner le curseur aprs le MultiCell

		// IMPORTANT: Revenir à la ligne si la cellule Assets a pris plusieurs lignes
		_, h := pdf.GetPageSize()
		if pdf.GetY() > h- { // Simple vrification de page break
			pdf.AddPage()
		}

		pdf.Ln() // Nouvelle ligne
	}

	// . Envoi du fichier
	c.Context().Response.Header.Set("Content-Type", "application/pdf")
	c.Context().Response.Header.Set("Content-Disposition", "attachment; filename=openrisk_report.pdf")

	return pdf.Output(c.Context().Response.BodyWriter())
}
