package company

type CompanyType string

const (
	TypeCorporations       CompanyType = "Corporations"
	TypeNonProfit          CompanyType = "NonProfit"
	TypeCooperative        CompanyType = "Cooperative"
	TypeSoleProprietorship CompanyType = "Sole Proprietorship"
)

func (t CompanyType) IsValid() bool {
	switch t {
	case TypeCorporations, TypeNonProfit, TypeCooperative, TypeSoleProprietorship:
		return true
	default:
		return false
	}
}
