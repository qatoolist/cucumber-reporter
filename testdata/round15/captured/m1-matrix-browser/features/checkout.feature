@checkout
Feature: Checkout 🛒 — Ünïcödé café
  As a **shopper** I want to _pay_ so that:
  - items ship
  - `code` blocks render

  | not | a | table |

  [docs](http://example.invalid/checkout)

  Background:
    Given the store "Café 東京 ☕" is open

  @smoke
  Scenario: Add one item
    When I add 1 items
    Then the cart has 1 items

  Scenario Outline: Add <n> items (<label>)
    When I add <n> items
    Then the cart has <expected> items

    Examples: small
      | n | expected | label |
      | 1 | 1 | row-1 |
      | 2 | 2 | row-2 |
      | 3 | 3 | row-3 |
      | 4 | 4 | row-4 |
      | 5 | 5 | row-5 |
      | 6 | 6 | row-6 |
      | 7 | 8 | row-7 wrong |
      | 8 | 8 | row-8 |
      | 9 | 9 | row-9 |
      | 10 | 10 | row-10 |

    @slow
    Examples: big
      | n | expected | label |
      | 10 | 10 | big-1 ✓ |
      | 20 | 20 | big-2 ✓ |
      | 30 | 30 | big-3 ✓ |
      | 40 | 40 | big-4 ✓ |
      | 50 | 50 | big-5 ✓ |

  Rule: Discounts 💸
    Background:
      Given I am logged in as "zoë"

    Scenario: Apply coupons from a data table
      When I apply coupons:
        | code   | pct |
        | SAVE5  | 5   |
        | SAVE10 | 5   |
      Then the total is 90

    Scenario: Invalid coupon has ANSI colour in the error
      When I apply an invalid coupon "BAD<1>"
      Then the total is 100

    Scenario: Receipt doc string
      Then the receipt is:
        """json
        {"total": 90, "emoji": "🎉", "note": "x < y & z"}
        """

  Scenario: Same name in two features
    When I add 3 items
    Then the cart has 3 items

  Scenario: Very long scenario name segment0 segment1 segment2 segment3 segment4 segment5 segment6 segment7 segment8 segment9 segment10 segment11 segment12 segment13 segment14 segment15 segment16 segment17 segment18 segment19 segment20 segment21 segment22 segment23 segment24 segment25 segment26 segment27 segment28 segment29 segment30 segment31 segment32 segment33 segment34 segment35 segment36 segment37 segment38 segment39 segment40 segment41 segment42 segment43 segment44 segment45 segment46 segment47 segment48 segment49 segment50 segment51 segment52 segment53 segment54 segment55 segment56 segment57 segment58 segment59 — end
    When I add 1 items
    Then the cart has 1 items

  @flaky
  Scenario: Payment gateway (chrome-ok, firefox-broken)
    When the payment gateway responds for "gw-firefox"

  @flaky
  Scenario: Payment gateway retried
    When the payment gateway responds for "gw-retry"
