package handlers

import (
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// ExportRisksPDF génère un rapport PDF de tous les risques actifs.
func ExportRisksPDF(c *fiber.Ctx) error {
	var risks []domain.Risk

	// 1. Récupérer les données
	if err := database.DB.Preload("Assets").Find(&risks).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch risks for export"})
	}

	// 2. Initialisation du PDF
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.SetFillColor(240, 240, 240) // Gris clair

	// 3. Titre et Métadonnées
	pdf.CellFormat(190, 10, "OpenRisk - Rapport d'Évaluation des Risques", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(190, 6, fmt.Sprintf("Date du rapport: %s", time.Now().Format("02 Jan 2006")), "", 1, "C", false, 0, "")
	pdf.Ln(8)

	// 4. En-tête du tableau
	header := []string{"Score", "Titre du Risque", "Impact", "Proba", "Assets Impactés"}
	colWidths := []float64{15, 85, 20, 20, 50}

	pdf.SetFont("Arial", "B", 10)
	for i, h := range header {
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// 5. Contenu du tableau
	pdf.SetFont("Arial", "", 9)
	for _, risk := range risks {
		// Assets list (simple string)
		assetNames := ""
		for i, asset := range risk.Assets {
			assetNames += asset.Name
			if i < len(risk.Assets)-1 {
				assetNames += ", "
			}
		}

		// Pour la lisibilité, les cellules sont définies par ligne
		pdf.CellFormat(colWidths[0], 6, fmt.Sprintf("%.2f", risk.Score), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[1], 6, risk.Title, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[2], 6, fmt.Sprintf("%d", risk.Impact), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[3], 6, fmt.Sprintf("%d", risk.Probability), "1", 0, "C", false, 0, "")

		// Gérer le cas où le texte des assets est trop long (multi-line)
		x, y := pdf.GetXY()
		pdf.MultiCell(colWidths[4], 6, assetNames, "1", "L", false)
		pdf.SetXY(x+colWidths[4], y) // Repositionner le curseur après le MultiCell

		// IMPORTANT: Revenir à la ligne si la cellule Assets a pris plusieurs lignes
		_, h := pdf.GetPageSize()
		if pdf.GetY() > h-20 { // Simple vérification de page break
			pdf.AddPage()
		}

		pdf.Ln(6) // Nouvelle ligne
	}

	// 6. Envoi du fichier
	c.Context().Response.Header.Set("Content-Type", "application/pdf")
	c.Context().Response.Header.Set("Content-Disposition", "attachment; filename=openrisk_report.pdf")

	return pdf.Output(c.Context().Response.BodyWriter())
}
