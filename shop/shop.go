package shop

import (
	"bufio"
	"fmt"
	"game/items"
	"game/player"
	"os"
	"strconv"
	"strings"
)

type Shop struct {
	Items []*items.Item
}

func NewShop() *Shop {
	return &Shop{
		Items: items.GetAllItems(),
	}
}

func (s *Shop) Visit(p *player.Player) {
	for {
		fmt.Println("\n=== 🏪 ЛАВКА ВООБРАЖЕНИЯ ===")
		fmt.Printf("💰 Ваше воображение: %d\n", p.Imagination)
		fmt.Println("============================")
		
		// Показываем товары
		for i, item := range s.Items {
			color := item.GetRarityColor()
			fmt.Printf("%s%d. %s - %d✨\033[0m\n", color, i+1, item.Name, item.Price)
			fmt.Printf("   └─ %s\n", item.Description)
		}
		
		fmt.Println("\n0. Выйти из магазина")
		fmt.Print("Выберите товар для покупки: ")
		
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Неверный ввод!")
			continue
		}
		
		if choice == 0 {
			break
		}
		
		if choice < 1 || choice > len(s.Items) {
			fmt.Println("Неверный номер товара!")
			continue
		}
		
		selectedItem := s.Items[choice-1]
		s.BuyItem(p, selectedItem)
	}
}

func (s *Shop) BuyItem(p *player.Player, item *items.Item) {
	if p.Imagination < item.Price {
		fmt.Println("❌ Недостаточно воображения!")
		return
	}
	
	if p.SpendImagination(item.Price) {
		// Создаем копию предмета
		itemCopy := &items.Item{
			Name:        item.Name,
			Description: item.Description,
			Rarity:      item.Rarity,
			Effect:      item.Effect,
			Price:       item.Price,
		}
		p.AddItem(itemCopy)
		fmt.Printf("✅ Куплено: %s\n", item.Name)
	}
}