package combat

type BodyPart int

const (
    Head BodyPart = iota
    Torso
    Legs
    Stun // Специальное значение для оглушения
    Negotiate // Специальное значение для переговоров
)

func (b BodyPart) String() string {
    switch b {
    case Head:
        return "голову"
    case Torso:
        return "тело"
    case Legs:
        return "ноги"
    case Stun:
        return "оглушение"
    case Negotiate:
        return "переговоры"
    default:
        return "неизвестно"
    }
}