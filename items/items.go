package items

type Rarity string

const (
	Common    Rarity = "common"
	Rare      Rarity = "rare"
	Legendary Rarity = "legendary"
)

type ItemEffect struct {
	Heal          int
	Strength      int
	MaxHP         int
	Imagination   int
	StunRounds    int
	SpecialEffect string
}

type Item struct {
	Name        string
	Description string
	Rarity      Rarity
	Effect      ItemEffect
	Price       int
}

func GetAllItems() []*Item {
	return []*Item{
		// Лечение
		{
			Name:        "🍵 Чай лунного сада",
			Description: "Серебристый настой. Лечит 40 HP",
			Rarity:      Common,
			Effect:      ItemEffect{Heal: 40},
			Price:       50,
		},
		{
			Name:        "🍯 Банка меда светлячков",
			Description: "Сладкий, слегка светится. Лечит 60 HP",
			Rarity:      Rare,
			Effect:      ItemEffect{Heal: 60},
			Price:       100,
		},
		{
			Name:        "🧃 Эликсир второго дыхания",
			Description: "Возвращает силы в самый критичный момент. Лечит 80 HP",
			Rarity:      Rare,
			Effect:      ItemEffect{Heal: 80},
			Price:       150,
		},

		// Усиление силы
		{
			Name:        "🗡 Деревянный меч фантазера",
			Description: "Легкий, но наполненный верой. +15 к силе",
			Rarity:      Common,
			Effect:      ItemEffect{Strength: 15},
			Price:       100,
		},
		{
			Name:        "🧤 Перчатки храбрости",
			Description: "Руки сами наносят удар увереннее. +25 к силе",
			Rarity:      Rare,
			Effect:      ItemEffect{Strength: 25},
			Price:       200,
		},
		{
			Name:        "🔥 Сердце дракончика",
			Description: "Горит внутри владельца. +40 к силе",
			Rarity:      Legendary,
			Effect:      ItemEffect{Strength: 40},
			Price:       250,
		},

		// Увеличение макс. HP
		{
			Name:        "🧥 Пальто из облаков",
			Description: "Легкое, но оберегает душу. +30 к макс. HP",
			Rarity:      Rare,
			Effect:      ItemEffect{MaxHP: 30},
			Price:       100,
		},
		{
			Name:        "🛡 Щит сказочного стража",
			Description: "Укрепляет тело и дух. +40 к макс. HP",
			Rarity:      Rare,
			Effect:      ItemEffect{MaxHP: 40},
			Price:       200,
		},
		{
			Name:        "Каменное сердце великана",
			Description: "Делает владельца почти несокрушимым. +70 к макс. HP",
			Rarity:      Legendary,
			Effect:      ItemEffect{MaxHP: 70},
			Price:       250,
		},
	}
}

func (i *Item) GetRarityColor() string {
	switch i.Rarity {
	case Common:
		return "\033[37m" 
	case Rare:
		return "\033[36m" 
	case Legendary:
		return "\033[33m" 
	default:
		return "\033[0m"
	}
}