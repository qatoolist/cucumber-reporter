# language: fr
@fr
Fonctionnalité: Panier en français — àéîõü
  Description avec **gras**.

  Contexte:
    Soit le magasin "Boulangerie Élise" est ouvert

  Scénario: Ajouter deux articles
    Quand j'ajoute 2 articles
    Alors le panier contient 2 articles

  Plan du scénario: Ajouter <n> articles
    Quand j'ajoute <n> articles
    Alors le panier contient <attendu> articles

    Exemples:
      | n | attendu |
      | 1 | 1       |
      | 2 | 3       |
      | 4 | 4       |

  Règle: Règle métier
    Scénario: Échec accentué
      Quand j'ajoute 1 articles
      Alors le panier contient 9 articles
