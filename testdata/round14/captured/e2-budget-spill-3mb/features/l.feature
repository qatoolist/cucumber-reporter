Feature: Budget spill
  Scenario: Passing with images
    Given I attach images 0 to 9

  Scenario: Failing with images
    Given I attach images 10 to 19
    And outcome "L"
