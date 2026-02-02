package main

import (
	"fmt"
	"log"

	"github.com/paularlott/scriptling"
)

func main() {
	// Create a new Scriptling interpreter
	p := scriptling.New()

	// Define a Counter class in Scriptling
	_, err := p.Eval(`
class Counter:
    """A simple counter class"""
    def __init__(self, start=0):
        self.value = start
    
    def increment(self, amount=1):
        """Increment the counter by amount"""
        self.value = self.value + amount
        return self.value
    
    def decrement(self, amount=1):
        """Decrement the counter by amount"""
        self.value = self.value - amount
        return self.value
    
    def get(self):
        """Get the current value"""
        return self.value
    
    def reset(self, new_value=0):
        """Reset the counter to a new value"""
        self.value = new_value
`)
	if err != nil {
		log.Fatalf("Failed to define class: %v", err)
	}

	// Create an instance of the Counter class from Go
	fmt.Println("Creating Counter instance with initial value 100...")
	instance, err := p.CreateInstance("Counter", 100)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}

	// Call methods on the instance from Go
	fmt.Println("\nCalling methods from Go:")

	// Get initial value
	result, _ := p.CallMethod(instance, "get")
	value, _ := result.AsInt()
	fmt.Printf("Initial value: %d\n", value)

	// Increment by default (1)
	result, _ = p.CallMethod(instance, "increment")
	value, _ = result.AsInt()
	fmt.Printf("After increment(): %d\n", value)

	// Increment by 10 using kwargs
	result, _ = p.CallMethod(instance, "increment", scriptling.Kwargs{"amount": 10})
	value, _ = result.AsInt()
	fmt.Printf("After increment(amount=10): %d\n", value)

	// Decrement by 5
	result, _ = p.CallMethod(instance, "decrement", scriptling.Kwargs{"amount": 5})
	value, _ = result.AsInt()
	fmt.Printf("After decrement(amount=5): %d\n", value)

	// Reset to 50
	p.CallMethod(instance, "reset", scriptling.Kwargs{"new_value": 50})
	result, _ = p.CallMethod(instance, "get")
	value, _ = result.AsInt()
	fmt.Printf("After reset(new_value=50): %d\n", value)

	// Store the instance in the environment for use in scripts
	fmt.Println("\nStoring instance in environment and using from script...")
	p.SetObjectVar("counter", instance)

	// Now use it from a script
	_, err = p.Eval(`
counter.increment(25)
final_value = counter.get()
print("Final value from script:", final_value)
`)
	if err != nil {
		log.Fatalf("Failed to execute script: %v", err)
	}

	// Retrieve the final value
	finalValue, _ := p.GetVarAsInt("final_value")
	fmt.Printf("Final value retrieved from Go: %d\n", finalValue)

	// Example with multiple instances
	fmt.Println("\n--- Multiple Instances Example ---")

	_, err = p.Eval(`
class BankAccount:
    def __init__(self, owner, balance=0):
        self.owner = owner
        self.balance = balance
    
    def deposit(self, amount):
        self.balance = self.balance + amount
        return self.balance
    
    def withdraw(self, amount):
        if amount <= self.balance:
            self.balance = self.balance - amount
            return self.balance
        else:
            return -1  # Insufficient funds
    
    def get_balance(self):
        return self.balance
    
    def get_owner(self):
        return self.owner
`)
	if err != nil {
		log.Fatalf("Failed to define BankAccount class: %v", err)
	}

	// Create two separate accounts
	account1, _ := p.CreateInstance("BankAccount", "Alice", scriptling.Kwargs{"balance": 1000})
	account2, _ := p.CreateInstance("BankAccount", "Bob", scriptling.Kwargs{"balance": 500})

	// Operate on account1
	result, _ = p.CallMethod(account1, "get_owner")
	owner, _ := result.AsString()
	fmt.Printf("\nAccount 1 owner: %s\n", owner)

	result, _ = p.CallMethod(account1, "get_balance")
	balance, _ := result.AsInt()
	fmt.Printf("Account 1 initial balance: $%d\n", balance)

	p.CallMethod(account1, "deposit", 250)
	result, _ = p.CallMethod(account1, "get_balance")
	balance, _ = result.AsInt()
	fmt.Printf("Account 1 after deposit $250: $%d\n", balance)

	// Operate on account2
	result, _ = p.CallMethod(account2, "get_owner")
	owner, _ = result.AsString()
	fmt.Printf("\nAccount 2 owner: %s\n", owner)

	result, _ = p.CallMethod(account2, "get_balance")
	balance, _ = result.AsInt()
	fmt.Printf("Account 2 initial balance: $%d\n", balance)

	p.CallMethod(account2, "withdraw", 100)
	result, _ = p.CallMethod(account2, "get_balance")
	balance, _ = result.AsInt()
	fmt.Printf("Account 2 after withdraw $100: $%d\n", balance)

	fmt.Println("\n✓ All operations completed successfully!")
}
