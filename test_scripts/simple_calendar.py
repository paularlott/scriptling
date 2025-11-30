def test():
    lines = []
    day = 1
    num_days = 28
    while day <= num_days:
        week_line = ""
        for i in range(7):
            if day <= num_days:
                week_line += str(day) + " "
                day += 1
            else:
                week_line += "   "
        lines.append(week_line)
    return "\n".join(lines)

print(test())

assert True