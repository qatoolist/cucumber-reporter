Feature: Large evidence
  Scenario: Passing with images
    Given I attach images 0 to 9

  Scenario: Failing with images and text
    Given I attach images 10 to 19
    And I attach the big text
    And outcome "L"
