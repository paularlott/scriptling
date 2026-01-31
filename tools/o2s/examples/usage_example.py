#!/usr/bin/env scriptling
"""
Example: Using a generated API client library
"""

# This would import the generated library
# import api_client

# Create a client instance
# client = api_client.APIClient("https://api.petstore.example.com/v1", "your-api-token-here")

# Or configure after creation
# client = api_client.APIClient("https://api.petstore.example.com/v1")
# client.set_auth_token("your-api-token-here")

# List all pets
# response = client.listPets(limit=10)
# if "error" not in response:
#     print("Status:", response["status"])
#     print("Pets:", response["body"])
# else:
#     print("Error:", response["error"])

# Get a specific pet
# response = client.getPet("pet-123")
# print(response)

# Create a new pet
# new_pet = {
#     "name": "Fluffy",
#     "species": "cat"
# }
# response = client.createPet(body=new_pet)
# print("Created:", response)

# Update a pet
# updated_pet = {
#     "name": "Fluffy the Great",
#     "species": "cat"
# }
# response = client.updatePet("pet-123", body=updated_pet)
# print("Updated:", response)

# Delete a pet
# response = client.deletePet("pet-123")
# print("Deleted:", response)

# Multiple environments
# prod = api_client.APIClient("https://prod.petstore.com", "prod-token")
# dev = api_client.APIClient("https://dev.petstore.com", "dev-token")
# prod_pets = prod.listPets()
# dev_pets = dev.listPets()

print("Example usage script - uncomment lines to use with generated library")
